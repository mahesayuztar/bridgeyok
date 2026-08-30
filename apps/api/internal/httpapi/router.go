package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/httpapi/apigen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

const requestIDHeader = "X-Request-ID"

type ReadinessChecker interface {
	Ping(context.Context) error
}

type Options struct {
	Logger         *slog.Logger
	AllowedOrigins []string
	Readiness      ReadinessChecker
	Identity       IdentityService
	Table          TableService
	Realtime       RealtimeService
}

type RealtimeService interface {
	http.Handler
	TableChanged(context.Context, string)
}

type IdentityService interface {
	CreateSession(context.Context, string) (identity.CredentialSet, error)
	Refresh(context.Context, string) (identity.CredentialSet, error)
	Authenticate(context.Context, string) (identity.Session, error)
	Revoke(context.Context, string) error
	IssueTicket(context.Context, identity.Session) (string, time.Time, error)
}

type TableService interface {
	Create(context.Context, identity.Session) (table.CreatedTable, error)
	Preview(context.Context, string) (table.Preview, error)
	Join(context.Context, string, identity.Session) (table.Projection, error)
	Get(context.Context, string, identity.Session) (table.Projection, error)
	Leave(context.Context, string, identity.Session) error
}

type contextKey string

const requestIDKey contextKey = "request_id"

func NewRouter(options Options) http.Handler {
	router := chi.NewRouter()
	router.Use(requestIDMiddleware)
	router.Use(requestLoggingMiddleware(options.Logger))
	router.Use(recoveryMiddleware(options.Logger))
	router.Use(securityHeadersMiddleware)
	router.Use(corsMiddleware(options.AllowedOrigins))

	router.Get("/", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, apigen.HealthResponse{Status: apigen.Ok, Service: "bridgeyok-api"})
	})
	router.Get("/health/live", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, apigen.HealthResponse{Status: apigen.Ok, Service: "api"})
	})
	router.Get("/health/ready", func(writer http.ResponseWriter, request *http.Request) {
		if options.Readiness == nil || options.Readiness.Ping(request.Context()) != nil {
			writeJSON(writer, http.StatusServiceUnavailable, apigen.HealthResponse{Status: apigen.Unavailable, Service: "api"})
			return
		}
		writeJSON(writer, http.StatusOK, apigen.HealthResponse{Status: apigen.Ready, Service: "api"})
	})
	identityHandler := identityHTTPHandler{service: options.Identity, logger: options.Logger}
	tableHandler := tableHTTPHandler{service: options.Table, identity: identityHandler, realtime: options.Realtime, logger: options.Logger}
	router.Post("/v1/guest-sessions", identityHandler.createSession)
	router.Post("/v1/guest-sessions/refresh", identityHandler.refreshSession)
	router.Delete("/v1/guest-sessions/current", identityHandler.revokeSession)
	router.Post("/v1/realtime/tickets", identityHandler.createRealtimeTicket)
	if options.Realtime != nil {
		router.Get("/v1/ws", options.Realtime.ServeHTTP)
	}
	router.Post("/v1/tables", tableHandler.createTable)
	router.Get("/v1/tables/{inviteCode}/preview", tableHandler.previewTable)
	router.Post("/v1/tables/{inviteCode}/join", tableHandler.joinTable)
	router.Get("/v1/tables/{tableId}", tableHandler.getTable)
	router.Post("/v1/tables/{tableId}/leave", tableHandler.leaveTable)
	router.NotFound(func(writer http.ResponseWriter, request *http.Request) {
		writeProblem(writer, request, http.StatusNotFound, "Resource not found")
	})
	router.MethodNotAllowed(func(writer http.ResponseWriter, request *http.Request) {
		writeProblem(writer, request, http.StatusMethodNotAllowed, "Method not allowed")
	})

	return router
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := request.Header.Get(requestIDHeader)
		if !validRequestID(requestID) {
			requestID = "req_" + rand.Text()
		}
		writer.Header().Set(requestIDHeader, requestID)
		ctx := context.WithValue(request.Context(), requestIDKey, requestID)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func validRequestID(requestID string) bool {
	if len(requestID) == 0 || len(requestID) > 64 {
		return false
	}
	for _, character := range requestID {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func requestLoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			startedAt := time.Now()
			wrappedWriter := middleware.NewWrapResponseWriter(writer, request.ProtoMajor)
			next.ServeHTTP(wrappedWriter, request)
			logger.InfoContext(request.Context(), "http request completed",
				"request_id", requestIDFromContext(request.Context()),
				"method", request.Method,
				"path", request.URL.Path,
				"status", wrappedWriter.Status(),
				"bytes", wrappedWriter.BytesWritten(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		})
	}
}

func recoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(request.Context(), "http request panic recovered",
						"request_id", requestIDFromContext(request.Context()),
						"panic_type", fmt.Sprintf("%T", recovered),
						"stack", string(debug.Stack()),
					)
					writeProblem(writer, request, http.StatusInternalServerError, "Internal server error")
				}
			}()
			next.ServeHTTP(writer, request)
		})
	}
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(writer, request)
	})
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			origin := request.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(writer, request)
				return
			}
			writer.Header().Add("Vary", "Origin")
			if !slices.Contains(allowedOrigins, origin) {
				writeProblem(writer, request, http.StatusForbidden, "Origin not allowed")
				return
			}
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			if request.Method == http.MethodOptions {
				writer.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey).(string)
	return requestID
}

func writeJSON(writer http.ResponseWriter, status int, response any) {
	if writer.Header().Get("Content-Type") == "" {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(response)
}

func writeProblem(writer http.ResponseWriter, request *http.Request, status int, title string) {
	requestID := requestIDFromContext(request.Context())
	writeJSON(writer, status, apigen.Problem{
		Type:      "about:blank",
		Title:     title,
		Status:    status,
		RequestId: &requestID,
	})
}
