package table

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

func TestDecideBotSeatLifecycle(t *testing.T) {
	t.Parallel()

	aggregate := testAggregateWithGuests(t, 2)
	guest := aggregate.Participants[1]
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandTakeSeat, SessionID: guest.SessionID, Seat: bridge.East}).NextState

	_, domainError := Decide(aggregate, Command{Name: CommandAddBot, SessionID: guest.SessionID, Seat: bridge.North, BotID: "bot-north"})
	assertDomainError(t, domainError, ErrorOwnerRequired)

	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandAddBot, SessionID: aggregate.OwnerSessionID, Seat: bridge.North, BotID: "bot-north"}).NextState
	if assignment := aggregate.Seats[bridge.North]; !assignment.IsBot || !assignment.Ready {
		t.Fatalf("bot assignment = %+v", assignment)
	}

	aggregate = acceptedDecision(t, aggregate, Command{
		Name: CommandReplaceWithBot, SessionID: aggregate.OwnerSessionID, ParticipantID: guest.ID,
		BotID: "bot-east", OccurredAt: testJoinedAt.Add(time.Minute),
	}).NextState
	if assignment := aggregate.Seats[bridge.East]; !assignment.IsBot || assignment.ParticipantID != "bot-east" {
		t.Fatalf("replacement bot assignment = %+v", assignment)
	}
	if _, active := aggregate.activeParticipant(guest.SessionID); active {
		t.Fatal("replaced participant remained active")
	}

	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRemoveBot, SessionID: aggregate.OwnerSessionID, Seat: bridge.North}).NextState
	if _, occupied := aggregate.Seats[bridge.North]; occupied {
		t.Fatal("removed bot retained its seat")
	}
}

func TestNextBotCommandUsesFirstLegalCall(t *testing.T) {
	t.Parallel()

	aggregate := testAggregateWithGuests(t, 2)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandAddBot, SessionID: aggregate.OwnerSessionID, Seat: bridge.North, BotID: "bot-north"}).NextState
	for _index, participant := range aggregate.Participants {
		seat := []bridge.Seat{bridge.East, bridge.South, bridge.West}[_index]
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandTakeSeat, SessionID: participant.SessionID, Seat: seat}).NextState
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandSetReady, SessionID: participant.SessionID, Ready: true}).NextState
	}
	deal := testDeal(t)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandStartGame, SessionID: aggregate.OwnerSessionID, Deal: &deal, BoardID: "board-one"}).NextState

	command, ready := nextBotCommand(aggregate)
	if !ready || command.Name != CommandMakeCall || command.BotSeat != bridge.North || command.Call == nil || *command.Call != bridge.Pass() {
		t.Fatalf("nextBotCommand() = %+v, %t", command, ready)
	}
}

func TestNextBotCommandUsesFirstLegalCard(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	for _, call := range []bridge.Call{bridge.Bid(1, bridge.StrainClubs), bridge.Pass(), bridge.Pass(), bridge.Pass()} {
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn), Call: &call}).NextState
	}
	openingLeader := aggregate.Game.Turn
	target := aggregate.Seats[openingLeader]
	aggregate = acceptedDecision(t, aggregate, Command{
		Name: CommandReplaceWithBot, SessionID: aggregate.OwnerSessionID, ParticipantID: target.ParticipantID,
		BotID: "bot-opening-leader", OccurredAt: testJoinedAt.Add(time.Minute),
	}).NextState
	legalCards, domainError := aggregate.Game.LegalCards(openingLeader)
	if domainError != nil || len(legalCards) == 0 {
		t.Fatalf("LegalCards() cards = %d, error = %v", len(legalCards), domainError)
	}

	command, ready := nextBotCommand(aggregate)
	if !ready || command.Name != CommandPlayCard || command.BotSeat != openingLeader || command.Card == nil || *command.Card != legalCards[0] {
		t.Fatalf("nextBotCommand() = %+v, %t", command, ready)
	}
}

func TestBotConsensusTransitions(t *testing.T) {
	t.Parallel()
	seats := []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West}
	for _, kind := range []ActionRequestKind{ActionRequestClaim, ActionRequestUndo} {
		for _, requester := range seats {
			others := []bridge.Seat{requester.Next(), requester.Partner(), requester.Partner().Next()}
			for _botMask := 0; _botMask < 8; _botMask++ {
				bots := []bridge.Seat{}
				for _index, seat := range others {
					if _botMask&(1<<_index) != 0 {
						bots = append(bots, seat)
					}
				}
				for _, rejectSeat := range append([]bridge.Seat{""}, others...) {
					if rejectSeat != "" && (slices.Contains(bots, rejectSeat) || kind == ActionRequestClaim && rejectSeat.Partnership() == requester.Partnership()) {
						continue
					}
					for _, botsFirst := range []bool{true, false} {
						t.Run(fmt.Sprintf("%s/%s/bots%d/reject%s/botsFirst%t", kind, requester, _botMask, rejectSeat, botsFirst), func(t *testing.T) {
							aggregate := botConsensusAggregate(t, kind, requester, bots)
							before := aggregate.Game.Clone()
							undoGame := aggregate.UndoableAction.Game.Clone()
							requestName, responseName := CommandRequestClaim, CommandRespondClaim
							if kind == ActionRequestUndo {
								requestName, responseName = CommandRequestUndo, CommandRespondUndo
							}
							aggregate = acceptedDecision(t, aggregate, Command{Name: requestName, SessionID: aggregate.OwnerSessionID, ClaimTricks: 5}).NextState
							terminalEvents := 0
							var lastCommand Command
							for _step := 0; aggregate.ActionRequest != nil && _step < 4; _step++ {
								encoded, err := json.Marshal(aggregate)
								if err != nil {
									t.Fatal(err)
								}
								var recovered Aggregate
								if err := json.Unmarshal(encoded, &recovered); err != nil {
									t.Fatal(err)
								}
								if err := recovered.Validate(); err != nil {
									t.Fatal(err)
								}
								aggregate = recovered
								botCommand, botReady := nextBotCommand(aggregate)
								var humanSeat bridge.Seat
								for _index := len(others) - 1; _index >= 0; _index-- {
									seat := others[_index]
									if aggregate.Seats[seat].IsBot || slices.Contains(aggregate.ActionRequest.ApprovedBy, seat) || kind == ActionRequestClaim && seat.Partnership() == requester.Partnership() {
										continue
									}
									humanSeat = seat
									break
								}
								if botReady && (botsFirst || humanSeat == "") {
									lastCommand = botCommand
									lastCommand.SessionID = aggregate.OwnerSessionID
								} else if humanSeat != "" {
									lastCommand = Command{Name: responseName, SessionID: sessionForSeat(t, aggregate, humanSeat), Accepted: humanSeat != rejectSeat}
								} else {
									t.Fatal("consensus has no possible next response")
								}
								decision := acceptedDecision(t, aggregate, lastCommand)
								for _, event := range decision.Events {
									if strings.HasSuffix(event.Type, "_ACCEPTED") || strings.HasSuffix(event.Type, "_REJECTED") {
										terminalEvents++
									}
								}
								aggregate = decision.NextState
							}
							if aggregate.ActionRequest != nil || terminalEvents != 1 {
								t.Fatalf("pending=%v terminal events=%d", aggregate.ActionRequest, terminalEvents)
							}
							wantRejected := rejectSeat != "" || aggregate.Seats[requester.Next()].IsBot && aggregate.Seats[requester.Partner().Next()].IsBot
							if wantRejected {
								if !reflect.DeepEqual(*aggregate.Game, before) {
									t.Fatal("rejection changed game")
								}
							} else if kind == ActionRequestUndo {
								if !reflect.DeepEqual(*aggregate.Game, undoGame) || aggregate.UndoableAction != nil {
									t.Fatal("undo did not restore prior state")
								}
							} else if aggregate.State != StateBetweenBoards || !aggregate.Game.Claimed || len(aggregate.ScoreSheet) != 1 {
								t.Fatal("claim did not produce one scored board")
							}
							if _, domainError := Decide(aggregate, lastCommand); domainError == nil {
								t.Fatal("late response produced a second outcome")
							}
						})
					}
				}
			}
		}
	}
}

func botConsensusAggregate(t *testing.T, kind ActionRequestKind, requester bridge.Seat, bots []bridge.Seat) Aggregate {
	t.Helper()
	aggregate := testReadyAggregate(t)
	aggregate.Seats[bridge.North], aggregate.Seats[requester] = aggregate.Seats[requester], aggregate.Seats[bridge.North]
	deal := testDeal(t)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandStartGame, SessionID: aggregate.OwnerSessionID, Deal: &deal, BoardID: "board-one"}).NextState
	pass := bridge.Pass()
	for aggregate.Game.Turn != requester {
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn), Call: &pass}).NextState
	}
	bid := bridge.Bid(1, bridge.StrainClubs)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: aggregate.OwnerSessionID, Call: &bid}).NextState
	if kind == ActionRequestClaim {
		for aggregate.Game.Phase == bridge.PhaseAuction {
			aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn), Call: &pass}).NextState
		}
		aggregate = playTableCards(t, aggregate, 4)
	}
	for _, seat := range bots {
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandReplaceWithBot, SessionID: aggregate.OwnerSessionID, ParticipantID: aggregate.Seats[seat].ParticipantID, BotID: "bot-" + string(seat), OccurredAt: testJoinedAt.Add(time.Minute)}).NextState
	}
	return aggregate
}

func TestBotConsensusRejectsPrematureAndForgedResponses(t *testing.T) {
	t.Parallel()
	aggregate := botConsensusAggregate(t, ActionRequestUndo, bridge.North, []bridge.Seat{bridge.East})
	projection, domainError := Project(aggregate, aggregate.OwnerSessionID)
	if domainError != nil || !projection.CanRequestUndo {
		t.Fatal("human latest actor cannot request undo with a bot seated")
	}
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRequestUndo, SessionID: aggregate.OwnerSessionID}).NextState
	for _, command := range []Command{
		{Name: CommandRespondUndo, SessionID: aggregate.OwnerSessionID, BotSeat: bridge.East, Accepted: true},
		{Name: CommandRespondUndo, SessionID: aggregate.OwnerSessionID, BotSeat: bridge.East, Accepted: false},
		{Name: CommandRequestClaim, SessionID: aggregate.OwnerSessionID, BotSeat: bridge.East},
		{Name: CommandRespondUndo, SessionID: sessionForSeat(t, aggregate, bridge.West), BotSeat: bridge.East, Accepted: true},
	} {
		if _, domainError := Decide(aggregate, command); domainError == nil {
			t.Fatalf("forged/premature bot command accepted: %+v", command)
		}
	}
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRespondUndo, SessionID: sessionForSeat(t, aggregate, bridge.West), Accepted: true}).NextState
	_, domainError = Decide(aggregate, Command{Name: CommandRespondUndo, SessionID: aggregate.OwnerSessionID, BotSeat: bridge.East, Accepted: false})
	assertDomainError(t, domainError, ErrorResponseNotAllowed)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRemoveBot, SessionID: aggregate.OwnerSessionID, Seat: bridge.East}).NextState
	if aggregate.ActionRequest != nil {
		t.Fatal("seat removal left a pending request")
	}
	_, domainError = Decide(aggregate, Command{Name: CommandRequestUndo, SessionID: aggregate.OwnerSessionID})
	assertDomainError(t, domainError, ErrorNotReady)
	projection, domainError = Project(aggregate, aggregate.OwnerSessionID)
	if domainError != nil || projection.CanRequestUndo {
		t.Fatal("empty seat exposed unanswerable undo")
	}
}
