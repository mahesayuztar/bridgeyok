package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/analysis"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type BoardReplayService interface {
	CompletedBoardReplay(context.Context, string, string) (table.BoardReplay, error)
}

type boardReplayHTTPHandler struct {
	service  BoardReplayService
	identity identityHTTPHandler
	logger   *slog.Logger
}

func (handler boardReplayHTTPHandler) get(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "private, no-store")
	session, ok := handler.identity.authenticate(writer, request)
	if !ok {
		return
	}
	boardID := request.PathValue("boardId")
	if _, err := uuid.Parse(boardID); err != nil {
		handler.identity.writeError(writer, request, http.StatusNotFound, "BOARD_NOT_FOUND", "replay.error.not_found", false)
		return
	}
	if handler.service == nil {
		handler.identity.writeError(writer, request, http.StatusServiceUnavailable, "REPLAY_UNAVAILABLE", "replay.error.unavailable", true)
		return
	}
	replay, err := handler.service.CompletedBoardReplay(request.Context(), boardID, session.ID)
	if err != nil {
		status, code, key := http.StatusServiceUnavailable, "REPLAY_UNAVAILABLE", "replay.error.unavailable"
		if errors.Is(err, analysis.ErrNotFound) {
			status, code, key = http.StatusNotFound, "BOARD_NOT_FOUND", "replay.error.not_found"
		}
		if errors.Is(err, analysis.ErrNotCompleted) {
			status, code, key = http.StatusConflict, "BOARD_NOT_COMPLETED", "replay.error.not_completed"
		}
		if handler.logger != nil {
			handler.logger.WarnContext(request.Context(), "board_replay_failed", "request_id", requestIDFromContext(request.Context()), "result_code", code)
		}
		handler.identity.writeError(writer, request, status, code, key, status >= 500)
		return
	}
	writeJSON(writer, http.StatusOK, replay)
}
