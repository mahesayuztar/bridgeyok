package history

import "context"

type Repository interface {
	ListHistoryBoards(context.Context, string, Cursor, int) ([]Board, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) List(ctx context.Context, sessionID, encodedCursor string, limit int) (Page, error) {
	cursor, err := DecodeCursor(encodedCursor)
	if err != nil {
		return Page{}, err
	}
	if err := ValidatePageSize(limit); err != nil {
		return Page{}, err
	}
	items, err := service.repository.ListHistoryBoards(ctx, sessionID, cursor, limit+1)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = EncodeCursor(Cursor{CompletedAt: last.CompletedAt, BoardID: last.BoardID})
	}
	return page, nil
}
