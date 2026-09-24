package history

import (
	"context"
	"strings"
)

type Repository interface {
	ListHistoryBoards(context.Context, string, Cursor, string, int) ([]Board, error)
}

type LabelRepository interface {
	SaveHistoryBoardLabel(context.Context, string, string, string) error
	DeleteHistoryBoardLabel(context.Context, string, string) error
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) List(ctx context.Context, sessionID, encodedCursor, search string, limit int) (Page, error) {
	search = strings.TrimSpace(search)
	if err := ValidateSearch(search); err != nil {
		return Page{}, err
	}
	cursor, err := DecodeCursor(encodedCursor)
	if err != nil {
		return Page{}, err
	}
	if err := ValidatePageSize(limit); err != nil {
		return Page{}, err
	}
	if encodedCursor != "" && cursor.Search != search {
		return Page{}, ErrInvalidCursor
	}
	items, err := service.repository.ListHistoryBoards(ctx, sessionID, cursor, search, limit+1)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = EncodeCursor(Cursor{CompletedAt: last.CompletedAt, BoardID: last.BoardID, Search: search})
	}
	return page, nil
}

func (service *Service) SetLabel(ctx context.Context, sessionID, boardID, label string) error {
	label = strings.TrimSpace(label)
	if err := ValidateLabel(label); err != nil {
		return err
	}
	repository, ok := service.repository.(LabelRepository)
	if !ok {
		return ErrHistoryBoardNotFound
	}
	return repository.SaveHistoryBoardLabel(ctx, sessionID, boardID, label)
}

func (service *Service) DeleteLabel(ctx context.Context, sessionID, boardID string) error {
	repository, ok := service.repository.(LabelRepository)
	if !ok {
		return ErrHistoryBoardNotFound
	}
	return repository.DeleteHistoryBoardLabel(ctx, sessionID, boardID)
}
