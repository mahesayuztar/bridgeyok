package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/httpapi/apigen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
)

const maxHTTPBodyBytes = 8 << 10

type identityHTTPHandler struct {
	service IdentityService
	logger  *slog.Logger
}

func (handler identityHTTPHandler) createSession(writer http.ResponseWriter, request *http.Request) {
	if handler.service == nil {
		handler.writeError(writer, request, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "common.error.service_unavailable", true)
		return
	}
	var body apigen.CreateGuestSessionRequest
	if err := decodeJSON(writer, request, &body); err != nil {
		handler.writeError(writer, request, http.StatusBadRequest, "INVALID_REQUEST", "common.error.invalid_request", false)
		return
	}
	credentials, err := handler.service.CreateSession(request.Context(), body.Nickname)
	if errors.Is(err, identity.ErrInvalidNickname) {
		handler.writeError(writer, request, http.StatusBadRequest, "INVALID_NICKNAME", "identity.error.invalid_nickname", false)
		return
	}
	if err != nil {
		handler.logger.ErrorContext(request.Context(), "guest_session_created", "request_id", requestIDFromContext(request.Context()), "result_code", "INTERNAL_ERROR")
		handler.writeError(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", "common.error.internal", true)
		return
	}
	handler.logger.InfoContext(request.Context(), "guest_session_created", "request_id", requestIDFromContext(request.Context()), "result_code", "CREATED")
	handler.writeCredentials(writer, request, http.StatusCreated, credentials)
}

func (handler identityHTTPHandler) refreshSession(writer http.ResponseWriter, request *http.Request) {
	if handler.service == nil {
		handler.writeError(writer, request, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "common.error.service_unavailable", true)
		return
	}
	var body apigen.RefreshGuestSessionRequest
	if err := decodeJSON(writer, request, &body); err != nil {
		handler.writeError(writer, request, http.StatusBadRequest, "INVALID_REQUEST", "common.error.invalid_request", false)
		return
	}
	credentials, err := handler.service.Refresh(request.Context(), body.DeviceCredential)
	if errors.Is(err, identity.ErrInvalidCredential) || errors.Is(err, identity.ErrSessionInactive) {
		handler.writeError(writer, request, http.StatusUnauthorized, "INVALID_CREDENTIAL", "identity.error.invalid_credential", false)
		return
	}
	if err != nil {
		handler.writeError(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", "common.error.internal", true)
		return
	}
	handler.writeCredentials(writer, request, http.StatusOK, credentials)
}

func (handler identityHTTPHandler) revokeSession(writer http.ResponseWriter, request *http.Request) {
	session, ok := handler.authenticate(writer, request)
	if !ok {
		return
	}
	if err := handler.service.Revoke(request.Context(), session.ID); err != nil {
		handler.writeError(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", "common.error.internal", true)
		return
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (handler identityHTTPHandler) createRealtimeTicket(writer http.ResponseWriter, request *http.Request) {
	session, ok := handler.authenticate(writer, request)
	if !ok {
		return
	}
	ticket, expiresAt, err := handler.service.IssueTicket(request.Context(), session)
	if err != nil {
		handler.writeError(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", "common.error.internal", true)
		return
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	writeJSON(writer, http.StatusCreated, apigen.RealtimeTicket{Ticket: ticket, ExpiresAt: expiresAt})
}

func (handler identityHTTPHandler) authenticate(writer http.ResponseWriter, request *http.Request) (identity.Session, bool) {
	if handler.service == nil {
		handler.writeError(writer, request, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "common.error.service_unavailable", true)
		return identity.Session{}, false
	}
	authorization := request.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer ") || strings.Contains(strings.TrimPrefix(authorization, "Bearer "), " ") {
		handler.writeError(writer, request, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "identity.error.authentication_required", false)
		return identity.Session{}, false
	}
	session, err := handler.service.Authenticate(request.Context(), strings.TrimPrefix(authorization, "Bearer "))
	if err != nil {
		handler.writeError(writer, request, http.StatusUnauthorized, "INVALID_ACCESS_TOKEN", "identity.error.invalid_access_token", false)
		return identity.Session{}, false
	}
	return session, true
}

func (handler identityHTTPHandler) writeCredentials(writer http.ResponseWriter, request *http.Request, status int, credentials identity.CredentialSet) {
	sessionID, err := uuid.Parse(credentials.Session.ID)
	if err != nil {
		handler.writeError(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", "common.error.internal", true)
		return
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	writeJSON(writer, status, apigen.GuestCredentials{
		SessionId:        sessionID,
		Nickname:         credentials.Session.Nickname,
		AccessToken:      credentials.AccessToken,
		AccessExpiresAt:  credentials.AccessExpiresAt,
		DeviceCredential: credentials.DeviceCredential,
	})
}

func (handler identityHTTPHandler) writeError(writer http.ResponseWriter, request *http.Request, status int, code string, messageKey string, retryable bool) {
	requestID := requestIDFromContext(request.Context())
	writer.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	writer.Header().Set("Cache-Control", "private, no-store")
	writeJSON(writer, status, apigen.Problem{
		Type:       "about:blank",
		Title:      http.StatusText(status),
		Status:     status,
		RequestId:  &requestID,
		Code:       &code,
		MessageKey: &messageKey,
		Retryable:  &retryable,
	})
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(writer, request.Body, maxHTTPBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request must contain one JSON value")
	}
	return nil
}
