package database

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/match"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

// CreateMatchLobby creates fresh private rooms with unready assigned players; account locks serialize retries and invitation capacity.
func (postgres *Postgres) CreateMatchLobby(ctx context.Context, ownerSessionID string, request match.CreateRequest, occurredAt time.Time) (match.Stored, error) {
	if err := request.Validate(); err != nil {
		return match.Stored{}, err
	}
	request.Assignments = slices.Clone(request.Assignments)
	slices.SortFunc(request.Assignments, func(a, b match.SeatRequest) int {
		return strings.Compare(string(a.Room)+string(a.Seat), string(b.Room)+string(b.Seat))
	})
	encoded, err := json.Marshal(request)
	if err != nil {
		return match.Stored{}, err
	}
	hash := sha256.Sum256(encoded)
	var stored match.Stored
	err = pgx.BeginFunc(ctx, postgres.pool, func(tx pgx.Tx) error {
		queries := postgres.queries.WithTx(tx)
		userIDs := make([]string, 0, 8)
		for _, assignment := range request.Assignments {
			userIDs = append(userIDs, assignment.UserID)
		}
		accounts, err := queries.LockMatchPlayers(ctx, userIDs)
		if err != nil {
			return err
		}
		if len(accounts) != 8 {
			return match.ErrInvalid
		}
		if !slices.ContainsFunc(accounts, func(account dbgen.LockMatchPlayersRow) bool { return account.SessionID == ownerSessionID }) {
			return match.ErrForbidden
		}
		previous, err := queries.FindMatchCreateRequest(ctx, dbgen.FindMatchCreateRequestParams{OwnerSessionID: ownerSessionID, RequestID: request.RequestID})
		if err == nil {
			if !bytes.Equal(previous.RequestHash, hash[:]) {
				return match.ErrState
			}
			row, err := queries.LoadMatch(ctx, previous.MatchID)
			if err != nil {
				return err
			}
			stored, err = hydrateMatch(ctx, queries, row)
			return err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		players := make(map[string]dbgen.ResolveMatchPlayerRow, 8)
		for _, account := range accounts {
			player, err := queries.ResolveMatchPlayer(ctx, dbgen.ResolveMatchPlayerParams{ID: account.ID, ExpiresAt: timestamptz(occurredAt)})
			if errors.Is(err, pgx.ErrNoRows) {
				return match.ErrInvalid
			}
			if err != nil {
				return err
			}
			count, err := queries.CountPendingMatchInvitations(ctx, account.SessionID)
			if err != nil {
				return err
			}
			if count >= 4 {
				return match.ErrCapacity
			}
			players[account.ID] = player
		}
		matchID := uuid.NewString()
		openTableID, closedTableID := uuid.NewString(), uuid.NewString()
		boardIDs := make([]string, request.BoardCount)
		for _index := range boardIDs {
			boardIDs[_index] = uuid.NewString()
		}
		candidate, err := match.New(matchID, ownerSessionID, openTableID, closedTableID, boardIDs)
		if err != nil {
			return err
		}
		assignments := make([]match.Assignment, 0, 8)
		for _, room := range []match.Room{match.Open, match.Closed} {
			roomAssignments := make([]match.SeatRequest, 0, 4)
			for _, assignment := range request.Assignments {
				if assignment.Room != room {
					continue
				}
				roomAssignments = append(roomAssignments, assignment)
			}
			_ownerIndex := slices.IndexFunc(roomAssignments, func(assignment match.SeatRequest) bool { return players[assignment.UserID].SessionID == ownerSessionID })
			if _ownerIndex < 0 {
				_ownerIndex = slices.IndexFunc(roomAssignments, func(assignment match.SeatRequest) bool { return assignment.Seat == bridge.North })
			}
			tableID := openTableID
			if room == match.Closed {
				tableID = closedTableID
			}
			roomOwner := players[roomAssignments[_ownerIndex].UserID]
			aggregate, err := table.NewAggregate(tableID, table.Participant{ID: uuid.NewString(), SessionID: roomOwner.SessionID, Nickname: roomOwner.Nickname, Role: table.RoleOwner, JoinedAt: occurredAt})
			if err != nil {
				return err
			}
			inviteHash := make([]byte, 32)
			if _, err := rand.Read(inviteHash); err != nil {
				return err
			}
			if err := createTable(ctx, queries, table.CreateRecord{Aggregate: aggregate, InviteCodeHash: inviteHash, CreatedAt: occurredAt}); err != nil {
				return err
			}
			for _index, assignment := range roomAssignments {
				player := players[assignment.UserID]
				if _index != _ownerIndex {
					participant := table.Participant{ID: uuid.NewString(), SessionID: player.SessionID, Nickname: player.Nickname, Role: table.RoleParticipant, JoinedAt: occurredAt}
					command := table.Command{Name: table.CommandJoinTable, Participant: &participant}
					decision, domainError := table.Decide(aggregate, command)
					if domainError != nil {
						return domainError
					}
					if err := queries.CreateTableParticipant(ctx, dbgen.CreateTableParticipantParams{ID: participant.ID, TableID: tableID, SessionID: participant.SessionID, Role: string(participant.Role), JoinedAt: timestamptz(occurredAt)}); err != nil {
						return err
					}
					result, err := persistAcceptedDecision(ctx, queries, table.CommandRequest{TableID: tableID, SessionID: player.SessionID, RequestID: "match_join", Command: command}, aggregate, decision, occurredAt)
					if err != nil {
						return err
					}
					aggregate = result.Aggregate
				}
				command := table.Command{Name: table.CommandTakeSeat, SessionID: player.SessionID, Seat: assignment.Seat}
				decision, domainError := table.Decide(aggregate, command)
				if domainError != nil {
					return domainError
				}
				result, err := persistAcceptedDecision(ctx, queries, table.CommandRequest{TableID: tableID, SessionID: player.SessionID, RequestID: "match_seat", Command: command}, aggregate, decision, occurredAt)
				if err != nil {
					return err
				}
				aggregate = result.Aggregate
				assignments = append(assignments, match.Assignment{ParticipantID: player.SessionID, Room: room, Seat: bridge.Seat(assignment.Seat)})
			}
		}
		if err := candidate.Assign(ownerSessionID, assignments); err != nil {
			return err
		}
		if err := createMatch(ctx, queries, candidate.PrivateSnapshot(), occurredAt); err != nil {
			return err
		}
		if err := queries.InsertMatchCreateRequest(ctx, dbgen.InsertMatchCreateRequestParams{OwnerSessionID: ownerSessionID, RequestID: request.RequestID, RequestHash: hash[:], MatchID: matchID}); err != nil {
			return err
		}
		stored = match.Stored{Match: candidate}
		return nil
	})
	if err != nil {
		return match.Stored{}, err
	}
	return stored, nil
}

func (postgres *Postgres) ListMatchIDs(ctx context.Context, sessionID string) ([]string, error) {
	return postgres.queries.ListParticipantMatchIDs(ctx, sessionID)
}

// CancelMatch closes both waiting rooms atomically when an invited player declines; active matches cannot be cancelled.
func (postgres *Postgres) CancelMatch(ctx context.Context, matchID, actorID string, expectedRevision int64, occurredAt time.Time) (match.Stored, error) {
	var result match.Stored
	err := pgx.BeginFunc(ctx, postgres.pool, func(tx pgx.Tx) error {
		queries := postgres.queries.WithTx(tx)
		rooms, err := queries.ListMatchRooms(ctx, matchID)
		if err != nil {
			return err
		}
		if len(rooms) != 2 {
			return match.ErrNotFound
		}
		tables, err := lockMatchTables(ctx, queries, []string{rooms[0].TableID, rooms[1].TableID})
		if err != nil {
			return err
		}
		row, err := queries.LockMatch(ctx, matchID)
		if err != nil {
			return err
		}
		stored, err := hydrateMatch(ctx, queries, row)
		if err != nil {
			return err
		}
		if _, err := stored.Match.Project(actorID); err != nil {
			return match.ErrNotFound
		}
		if row.Status == string(match.Cancelled) {
			result = stored
			return nil
		}
		if row.Revision != expectedRevision {
			return match.ErrState
		}
		if err := stored.Match.Cancel(actorID); err != nil {
			return err
		}
		for _, aggregate := range tables {
			command := table.Command{Name: table.CommandFinishTable, SessionID: aggregate.OwnerSessionID}
			decision, domainError := table.Decide(aggregate, command)
			if domainError != nil {
				return domainError
			}
			if _, err := persistAcceptedDecision(ctx, queries, table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: "match_cancel", Command: command}, aggregate, decision, occurredAt); err != nil {
				return err
			}
		}
		if err := saveMatchProgress(ctx, queries, stored, occurredAt); err != nil {
			return err
		}
		stored.Revision++
		result = stored
		return nil
	})
	if err != nil {
		return match.Stored{}, err
	}
	return result, nil
}
