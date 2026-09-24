package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/history"
)

type HistoryService interface {
	List(context.Context, string, string, string, int) (history.Page, error)
}

type HistoryLabelService interface {
	SetLabel(context.Context, string, string, string) error
	DeleteLabel(context.Context, string, string) error
}

type historyHTTPHandler struct {
	service  HistoryService
	identity identityHTTPHandler
	logger   *slog.Logger
}

func (handler historyHTTPHandler) list(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "private, no-store")
	session, ok := handler.identity.authenticate(writer, request)
	if !ok {
		return
	}
	if handler.service == nil {
		handler.identity.writeError(writer, request, http.StatusServiceUnavailable, "HISTORY_UNAVAILABLE", "history.error.unavailable", true)
		return
	}
	limit := history.DefaultPageSize
	if value := request.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			handler.writeInvalid(writer, request)
			return
		}
		limit = parsed
	}
	search := request.URL.Query().Get("search")
	page, err := handler.service.List(request.Context(), session.ID, request.URL.Query().Get("cursor"), search, limit)
	if err != nil {
		if errors.Is(err, history.ErrInvalidCursor) || errors.Is(err, history.ErrInvalidPageSize) || errors.Is(err, history.ErrInvalidSearch) {
			handler.writeInvalid(writer, request)
			return
		}
		handler.logger.ErrorContext(request.Context(), "history_list_failed", "request_id", requestIDFromContext(request.Context()), "result_code", "INTERNAL_ERROR")
		handler.identity.writeError(writer, request, http.StatusServiceUnavailable, "HISTORY_UNAVAILABLE", "history.error.unavailable", true)
		return
	}
	writeJSON(writer, http.StatusOK, page)
}

func (handler historyHTTPHandler) setLabel(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "private, no-store")
	session, ok := handler.identity.authenticate(writer, request)
	if !ok {
		return
	}
	service, ok := handler.service.(HistoryLabelService)
	if !ok {
		handler.identity.writeError(writer, request, http.StatusServiceUnavailable, "HISTORY_UNAVAILABLE", "history.error.unavailable", true)
		return
	}
	boardID := chi.URLParam(request, "boardId")
	if _, err := uuid.Parse(boardID); err != nil {
		handler.writeInvalid(writer, request)
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	if decodeJSON(writer, request, &body) != nil {
		handler.writeInvalid(writer, request)
		return
	}
	label := strings.TrimSpace(body.Label)
	if err := service.SetLabel(request.Context(), session.ID, boardID, label); err != nil {
		handler.writeLabelError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, struct {
		Label string `json:"label"`
	}{Label: label})
}

func (handler historyHTTPHandler) deleteLabel(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "private, no-store")
	session, ok := handler.identity.authenticate(writer, request)
	if !ok {
		return
	}
	service, ok := handler.service.(HistoryLabelService)
	if !ok {
		handler.identity.writeError(writer, request, http.StatusServiceUnavailable, "HISTORY_UNAVAILABLE", "history.error.unavailable", true)
		return
	}
	boardID := chi.URLParam(request, "boardId")
	if _, err := uuid.Parse(boardID); err != nil {
		handler.writeInvalid(writer, request)
		return
	}
	if err := service.DeleteLabel(request.Context(), session.ID, boardID); err != nil {
		handler.writeLabelError(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (handler historyHTTPHandler) writeLabelError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, history.ErrInvalidLabel):
		handler.identity.writeError(writer, request, http.StatusBadRequest, "INVALID_HISTORY_LABEL", "history.error.invalid_label", false)
	case errors.Is(err, history.ErrHistoryBoardNotFound):
		handler.identity.writeError(writer, request, http.StatusNotFound, "HISTORY_BOARD_NOT_FOUND", "history.error.board_not_found", false)
	default:
		handler.logger.ErrorContext(request.Context(), "history_label_failed", "request_id", requestIDFromContext(request.Context()), "result_code", "INTERNAL_ERROR")
		handler.identity.writeError(writer, request, http.StatusServiceUnavailable, "HISTORY_UNAVAILABLE", "history.error.unavailable", true)
	}
}

func (handler historyHTTPHandler) writeInvalid(writer http.ResponseWriter, request *http.Request) {
	handler.identity.writeError(writer, request, http.StatusBadRequest, "INVALID_HISTORY_INPUT", "history.error.invalid_input", false)
}
