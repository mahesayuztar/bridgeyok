package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/chat"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
)

type accountHTTPHandler struct {
	service       *identity.Service
	repository    identity.AccountRepository
	tables        TableService
	identity      identityHTTPHandler
	passwordSlots chan struct{}
	realtime      RealtimeService
}

func (handler accountHTTPHandler) credentials(writer http.ResponseWriter, request *http.Request) {
	select {
	case handler.passwordSlots <- struct{}{}:
		defer func() { <-handler.passwordSlots }()
	default:
		handler.identity.writeError(writer, request, 429, "RATE_LIMITED", "account.error.rate_limited", true)
		return
	}
	var body struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
		Avatar      string `json:"avatar"`
	}
	if decodeJSON(writer, request, &body) != nil {
		handler.fail(writer, request, identity.ErrAccountInput)
		return
	}
	var login identity.AccountLogin
	var err error
	if strings.HasSuffix(request.URL.Path, "signup") {
		login, err = handler.service.Register(request.Context(), body.Username, body.Password, body.DisplayName, body.Avatar)
	} else {
		login, err = handler.service.Login(request.Context(), body.Username, body.Password)
	}
	if err != nil {
		handler.fail(writer, request, err)
		return
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	writeJSON(writer, http.StatusOK, login)
}

func (handler accountHTTPHandler) serve(writer http.ResponseWriter, request *http.Request) {
	token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	account, err := handler.service.Account(request.Context(), token)
	if err != nil {
		handler.fail(writer, request, err)
		return
	}
	writer.Header().Set("Cache-Control", "private, no-store")
	var result interface{}
	switch {
	case request.URL.Path == "/v1/account" && request.Method == http.MethodGet:
		result = account
	case request.URL.Path == "/v1/account/logout":
		err = handler.service.LogoutAccount(request.Context(), token)
	case request.URL.Path == "/v1/account/heartbeat":
		err = handler.service.AccountHeartbeat(request.Context(), token)
	case request.URL.Path == "/v1/account/profile":
		var body struct {
			DisplayName string `json:"displayName"`
			Avatar      string `json:"avatar"`
		}
		if decodeJSON(writer, request, &body) != nil {
			err = identity.ErrAccountInput
			break
		}
		result, err = handler.service.SaveProfile(request.Context(), account.Profile.ID, body.DisplayName, body.Avatar)
	case request.URL.Path == "/v1/account/users":
		query := strings.TrimSpace(request.URL.Query().Get("q"))
		if len(query) > 96 {
			err = identity.ErrAccountInput
			break
		}
		result, err = handler.repository.SearchUsers(request.Context(), account.Profile.ID, query, request.URL.Query().Get("friends") == "true", time.Now().UTC())
	case request.URL.Path == "/v1/account/chat":
		repository, ok := handler.repository.(chat.Repository)
		if !ok {
			err = chat.ErrAccess
			break
		}
		result, err = repository.ChatHistory(request.Context(), account.SessionID, chat.Target{Scope: request.URL.Query().Get("scope"), ID: request.URL.Query().Get("id")}, request.URL.Query().Get("cursor"), 50)
	case request.URL.Path == "/v1/account/invitations":
		result, err = handler.repository.Invitations(request.Context(), account.Profile.ID, time.Now().UTC())
	case chi.URLParam(request, "userId") != "":
		targetID := chi.URLParam(request, "userId")
		if _, parseErr := uuid.Parse(targetID); parseErr != nil {
			err = identity.ErrAccountInput
			break
		}
		if events, ok := handler.repository.(interface {
			FollowWithEvent(context.Context, string, string) (bool, bool, error)
		}); ok && request.Method == http.MethodPut {
			var changed, mutual bool
			changed, mutual, err = events.FollowWithEvent(request.Context(), account.Profile.ID, targetID)
			if err == nil && changed {
				handler.notify(request, targetID, "follow", account.Profile, map[string]any{})
				if mutual {
					handler.notify(request, targetID, "friend", account.Profile, map[string]any{})
				}
			}
		} else {
			err = handler.repository.Follow(request.Context(), account.Profile.ID, targetID, request.Method == http.MethodPut)
		}
	case chi.URLParam(request, "tableId") != "":
		tableID := chi.URLParam(request, "tableId")
		if _, parseErr := uuid.Parse(tableID); parseErr != nil {
			err = identity.ErrAccountInput
			break
		}
		session, authErr := handler.service.Authenticate(request.Context(), token)
		if authErr != nil {
			err = authErr
			break
		}
		projection, getErr := handler.tables.Get(request.Context(), tableID, session)
		if getErr != nil {
			handler.identity.writeError(writer, request, 403, "NOT_PARTICIPANT", "table.error.not_participant", false)
			return
		}
		if request.Method == http.MethodGet {
			result, err = handler.repository.ParticipantProfiles(request.Context(), account.Profile.ID, tableID, time.Now().UTC())
			break
		}
		var body struct {
			UserID     string `json:"userId"`
			InviteCode string `json:"inviteCode"`
		}
		if decodeJSON(writer, request, &body) != nil {
			err = identity.ErrAccountInput
			break
		}
		if _, parseErr := uuid.Parse(body.UserID); parseErr != nil {
			err = identity.ErrAccountInput
			break
		}
		if projection.Locked {
			err = identity.ErrUserOffline
			break
		}
		preview, previewErr := handler.tables.Preview(request.Context(), body.InviteCode)
		if previewErr != nil || preview.TableID != tableID {
			err = identity.ErrAccountInput
			break
		}
		err = handler.repository.InvitePlayer(request.Context(), account.Profile.ID, body.UserID, tableID, body.InviteCode, time.Now().UTC())
		if err == nil {
			handler.notify(request, body.UserID, "invite", account.Profile, map[string]any{"tableId": tableID, "inviteCode": body.InviteCode})
		}
	default:
		handler.identity.writeError(writer, request, 404, "NOT_FOUND", "common.error.not_found", false)
		return
	}
	if err != nil {
		handler.fail(writer, request, err)
		return
	}
	if result == nil {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (handler accountHTTPHandler) fail(writer http.ResponseWriter, request *http.Request, err error) {
	status, code := 500, "INTERNAL_ERROR"
	switch {
	case errors.Is(err, chat.ErrAccess):
		status, code = 403, "CHAT_ACCESS_DENIED"
	case errors.Is(err, chat.ErrInput):
		status, code = 400, "INVALID_CHAT_INPUT"
	case errors.Is(err, identity.ErrAccountRate):
		status, code = 429, "RATE_LIMITED"
	case errors.Is(err, identity.ErrAccountInput):
		status, code = 400, "INVALID_ACCOUNT_INPUT"
	case errors.Is(err, identity.ErrUsernameTaken):
		status, code = 409, "USERNAME_TAKEN"
	case errors.Is(err, identity.ErrInvalidCredential), errors.Is(err, identity.ErrSessionInactive):
		status, code = 401, "INVALID_CREDENTIAL"
	case errors.Is(err, identity.ErrUserOffline):
		status, code = 409, "USER_OFFLINE"
	}
	handler.identity.logger.InfoContext(request.Context(), "account_operation", "request_id", requestIDFromContext(request.Context()), "result_code", code)
	handler.identity.writeError(writer, request, status, code, "account.error."+strings.ToLower(code), status >= 500)
}

func (handler accountHTTPHandler) notify(request *http.Request, targetID, name string, sender identity.Profile, payload map[string]any) {
	repository, ok := handler.repository.(interface {
		SocialProfile(context.Context, string) (identity.Profile, string, error)
	})
	if !ok {
		return
	}
	realtime, ok := handler.realtime.(interface {
		NotifySession(string, string, string, map[string]any)
	})
	if !ok {
		return
	}
	_, sessionID, err := repository.SocialProfile(request.Context(), targetID)
	if err != nil {
		return
	}
	payload["sender"] = sender
	realtime.NotifySession(sessionID, uuid.NewString(), name, payload)
}
