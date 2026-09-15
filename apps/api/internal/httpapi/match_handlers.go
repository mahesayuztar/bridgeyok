package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/match"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type MatchService interface {
	Create(context.Context, string, match.CreateRequest) (match.PublicView, error)
	Get(context.Context, string, string) (match.PublicView, error)
	List(context.Context, string) ([]match.PublicView, error)
	Start(context.Context, string, string, int64) (match.PublicView, error)
	Ready(context.Context, string, string, string, int64, bool) (match.PublicView, error)
	Cancel(context.Context, string, string, int64) (match.PublicView, error)
}

type matchHTTPHandler struct {
	service  MatchService
	identity identityHTTPHandler
	logger   *slog.Logger
}

func (handler matchHTTPHandler) serve(writer http.ResponseWriter, request *http.Request) {
	if handler.service == nil {
		handler.writeError(writer, request, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", true)
		return
	}
	session, ok := handler.identity.authenticate(writer, request)
	if !ok {
		return
	}
	matchID := request.PathValue("matchId")
	var result any
	var err error
	status := http.StatusOK
	switch {
	case request.Method == http.MethodGet && matchID == "":
		result, err = handler.service.List(request.Context(), session.ID)
	case request.Method == http.MethodGet:
		result, err = handler.service.Get(request.Context(), matchID, session.ID)
	case matchID == "":
		var input match.CreateRequest
		if decodeJSON(writer, request, &input) != nil {
			handler.writeError(writer, request, 400, "INVALID_MATCH_INPUT", false)
			return
		}
		result, err = handler.service.Create(request.Context(), session.ID, input)
		status = http.StatusCreated
	case request.URL.Path == "/v1/matches/"+matchID+"/ready":
		var input struct {
			RequestID             string `json:"requestId"`
			ExpectedTableRevision *int64 `json:"expectedTableRevision"`
			Ready                 *bool  `json:"ready"`
		}
		if decodeJSON(writer, request, &input) != nil || input.ExpectedTableRevision == nil || input.Ready == nil {
			handler.writeError(writer, request, 400, "INVALID_MATCH_INPUT", false)
			return
		}
		result, err = handler.service.Ready(request.Context(), matchID, session.ID, input.RequestID, *input.ExpectedTableRevision, *input.Ready)
	default:
		var input struct {
			ExpectedRevision *int64 `json:"expectedRevision"`
		}
		if decodeJSON(writer, request, &input) != nil || input.ExpectedRevision == nil {
			handler.writeError(writer, request, 400, "INVALID_MATCH_INPUT", false)
			return
		}
		if request.URL.Path == "/v1/matches/"+matchID+"/start" {
			result, err = handler.service.Start(request.Context(), matchID, session.ID, *input.ExpectedRevision)
		} else {
			result, err = handler.service.Cancel(request.Context(), matchID, session.ID, *input.ExpectedRevision)
		}
	}
	if err != nil {
		switch {
		case errors.Is(err, match.ErrNotFound):
			handler.writeError(writer, request, 404, "MATCH_NOT_FOUND", false)
		case errors.Is(err, match.ErrInvalid):
			handler.writeError(writer, request, 400, "INVALID_MATCH_INPUT", false)
		case errors.Is(err, match.ErrForbidden):
			handler.writeError(writer, request, 403, "MATCH_FORBIDDEN", false)
		case errors.Is(err, match.ErrState):
			handler.writeError(writer, request, 409, "STATE_CHANGED", false)
		case errors.Is(err, match.ErrCapacity):
			handler.writeError(writer, request, 429, "MATCH_CAPACITY", true)
		case errors.Is(err, table.ErrActorQueueFull), errors.Is(err, table.ErrActorStopped), errors.Is(err, table.ErrActorRegistryDraining), errors.Is(err, deal.ErrUnavailable):
			handler.writeError(writer, request, 503, "SERVICE_UNAVAILABLE", true)
		default:
			handler.logger.ErrorContext(request.Context(), "match_operation_failed", "request_id", requestIDFromContext(request.Context()), "result_code", "INTERNAL_ERROR")
			handler.writeError(writer, request, 500, "INTERNAL_ERROR", true)
		}
		return
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	writeJSON(writer, status, result)
}

func (handler matchHTTPHandler) writeError(writer http.ResponseWriter, request *http.Request, status int, code string, retryable bool) {
	key := "match.error.unavailable"
	switch code {
	case "MATCH_NOT_FOUND":
		key = "match.error.not_found"
	case "INVALID_MATCH_INPUT":
		key = "match.error.invalid_input"
	case "MATCH_FORBIDDEN":
		key = "match.error.forbidden"
	case "STATE_CHANGED":
		key = "match.error.state_changed"
	case "MATCH_CAPACITY":
		key = "match.error.capacity"
	}
	handler.identity.writeError(writer, request, status, code, key, retryable)
}
