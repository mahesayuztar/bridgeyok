package history

import (
	"context"
	"errors"
	"testing"
	"time"
)

type historyRepositoryFake struct {
	items []Board
	limit int
}

func (fake *historyRepositoryFake) ListHistoryBoards(_ context.Context, _ string, _ Cursor, _ string, limit int) ([]Board, error) {
	fake.limit = limit
	return fake.items, nil
}

func TestCursorRoundTrip(t *testing.T) {
	cursor := Cursor{CompletedAt: time.Date(2026, 9, 24, 10, 30, 0, 0, time.UTC), BoardID: "board-id"}
	encoded := EncodeCursor(cursor)
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if !decoded.CompletedAt.Equal(cursor.CompletedAt) || decoded.BoardID != cursor.BoardID {
		t.Fatalf("decoded cursor = %+v, want %+v", decoded, cursor)
	}
}

func TestDecodeCursorRejectsInvalidValue(t *testing.T) {
	if _, err := DecodeCursor("invalid"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("error = %v, want invalid cursor", err)
	}
}

func TestServiceUsesOneLookaheadItemForNextCursor(t *testing.T) {
	completedAt := time.Date(2026, 9, 24, 10, 30, 0, 0, time.UTC)
	repository := &historyRepositoryFake{items: []Board{
		{BoardID: "first", CompletedAt: completedAt},
		{BoardID: "second", CompletedAt: completedAt.Add(-time.Minute)},
	}}
	service := NewService(repository)
	page, err := service.List(context.Background(), "session", "", "", 1)
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if repository.limit != 2 || len(page.Items) != 1 || page.Items[0].BoardID != "first" || page.NextCursor == "" {
		t.Fatalf("repository limit = %d, page = %+v", repository.limit, page)
	}
}

func TestServiceBindsCursorToSearch(t *testing.T) {
	completedAt := time.Date(2026, 9, 24, 10, 30, 0, 0, time.UTC)
	repository := &historyRepositoryFake{items: []Board{{BoardID: "first", CompletedAt: completedAt}, {BoardID: "second", CompletedAt: completedAt.Add(-time.Minute)}}}
	service := NewService(repository)
	page, err := service.List(context.Background(), "session", "", "finesse", 1)
	if err != nil || page.NextCursor == "" {
		t.Fatalf("list history = %+v, error = %v", page, err)
	}
	if _, err := service.List(context.Background(), "session", page.NextCursor, "other", 1); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("error = %v, want invalid cursor", err)
	}
}

func TestValidateLabel(t *testing.T) {
	if err := ValidateLabel("  finesse timing  "); err != nil {
		t.Fatalf("valid label rejected: %v", err)
	}
	if err := ValidateLabel(""); !errors.Is(err, ErrInvalidLabel) {
		t.Fatalf("empty label error = %v, want invalid label", err)
	}
}
