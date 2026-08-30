package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
)

// CreateSession persists a new guest credential verifier.
func (postgres *Postgres) CreateSession(ctx context.Context, record identity.SessionRecord) error {
	if err := postgres.queries.CreateGuestSession(ctx, dbgen.CreateGuestSessionParams{
		ID:             record.ID,
		CredentialHash: record.CredentialHash,
		Nickname:       record.Nickname,
		CreatedAt:      timestamptz(record.CreatedAt),
		LastSeenAt:     timestamptz(record.LastSeenAt),
		ExpiresAt:      timestamptz(record.ExpiresAt),
	}); err != nil {
		return fmt.Errorf("insert guest session: %w", err)
	}
	return nil
}

// RotateCredential replaces one active device verifier atomically.
func (postgres *Postgres) RotateCredential(ctx context.Context, oldHash []byte, newHash []byte, now time.Time) (identity.Session, error) {
	row, err := postgres.queries.RotateGuestCredential(ctx, dbgen.RotateGuestCredentialParams{
		NewCredentialHash: newHash,
		Now:               timestamptz(now),
		OldCredentialHash: oldHash,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Session{}, identity.ErrInvalidCredential
	}
	if err != nil {
		return identity.Session{}, fmt.Errorf("update guest credential: %w", err)
	}
	return identity.Session{ID: row.ID, Nickname: row.Nickname, Status: row.Status, ExpiresAt: row.ExpiresAt.Time}, nil
}

// FindActiveSession returns an unexpired, non-revoked guest.
func (postgres *Postgres) FindActiveSession(ctx context.Context, sessionID string, now time.Time) (identity.Session, error) {
	row, err := postgres.queries.FindActiveGuestSession(ctx, dbgen.FindActiveGuestSessionParams{ID: sessionID, Now: timestamptz(now)})
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Session{}, identity.ErrSessionInactive
	}
	if err != nil {
		return identity.Session{}, fmt.Errorf("select active guest session: %w", err)
	}
	return identity.Session{ID: row.ID, Nickname: row.Nickname, Status: row.Status, ExpiresAt: row.ExpiresAt.Time}, nil
}

// RevokeSession marks one active guest credential chain as unusable.
func (postgres *Postgres) RevokeSession(ctx context.Context, sessionID string, now time.Time) error {
	rows, err := postgres.queries.RevokeGuestSession(ctx, dbgen.RevokeGuestSessionParams{Now: timestamptz(now), ID: sessionID})
	if err != nil {
		return fmt.Errorf("update guest status: %w", err)
	}
	if rows != 1 {
		return identity.ErrSessionInactive
	}
	return nil
}

// StoreTicket persists a short-lived realtime ticket verifier.
func (postgres *Postgres) StoreTicket(ctx context.Context, ticketHash []byte, sessionID string, createdAt time.Time, expiresAt time.Time) error {
	if err := postgres.queries.StoreRealtimeTicket(ctx, dbgen.StoreRealtimeTicketParams{
		TicketHash: ticketHash,
		SessionID:  sessionID,
		CreatedAt:  timestamptz(createdAt),
		ExpiresAt:  timestamptz(expiresAt),
	}); err != nil {
		return fmt.Errorf("insert realtime ticket: %w", err)
	}
	return nil
}

// ConsumeTicket marks a ticket used and returns its active guest atomically.
func (postgres *Postgres) ConsumeTicket(ctx context.Context, ticketHash []byte, now time.Time) (identity.Session, error) {
	row, err := postgres.queries.ConsumeRealtimeTicket(ctx, dbgen.ConsumeRealtimeTicketParams{Now: timestamptz(now), TicketHash: ticketHash})
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.Session{}, identity.ErrInvalidTicket
	}
	if err != nil {
		return identity.Session{}, fmt.Errorf("consume realtime ticket: %w", err)
	}
	return identity.Session{ID: row.ID, Nickname: row.Nickname, Status: row.Status, ExpiresAt: row.ExpiresAt.Time}, nil
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
