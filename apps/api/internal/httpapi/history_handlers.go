package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/history"
)

type HistoryService interface {
	List(context.Context, string, string, int) (history.Page, error)
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
	page, err := handler.service.List(request.Context(), session.ID, request.URL.Query().Get("cursor"), limit)
	if err != nil {
		if errors.Is(err, history.ErrInvalidCursor) || errors.Is(err, history.ErrInvalidPageSize) {
			handler.writeInvalid(writer, request)
			return
		}
		handler.logger.ErrorContext(request.Context(), "history_list_failed", "request_id", requestIDFromContext(request.Context()), "result_code", "INTERNAL_ERROR")
		handler.identity.writeError(writer, request, http.StatusServiceUnavailable, "HISTORY_UNAVAILABLE", "history.error.unavailable", true)
		return
	}
	writeJSON(writer, http.StatusOK, page)
}

func (handler historyHTTPHandler) writeInvalid(writer http.ResponseWriter, request *http.Request) {
	handler.identity.writeError(writer, request, http.StatusBadRequest, "INVALID_HISTORY_INPUT", "history.error.invalid_input", false)
}
