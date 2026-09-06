package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/analysis"
)

type AnalysisService interface {
	Analyze(context.Context, string, string) (analysis.Response, error)
}

type analysisHTTPHandler struct {
	service  AnalysisService
	identity identityHTTPHandler
	logger   *slog.Logger
}

func (handler analysisHTTPHandler) analyze(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "private, no-store")
	session, ok := handler.identity.authenticate(writer, request)
	if !ok {
		return
	}
	if handler.service == nil {
		handler.identity.writeError(writer, request, http.StatusServiceUnavailable, "ANALYSIS_UNAVAILABLE", "analysis.error.unavailable", true)
		return
	}
	startedAt := time.Now()
	response, err := handler.service.Analyze(request.Context(), request.PathValue("boardId"), session.ID)
	code, status, key := "OK", http.StatusOK, ""
	switch {
	case errors.Is(err, analysis.ErrNotFound):
		code, status, key = "BOARD_NOT_FOUND", http.StatusNotFound, "analysis.error.not_found"
	case errors.Is(err, analysis.ErrNotCompleted):
		code, status, key = "BOARD_NOT_COMPLETED", http.StatusConflict, "analysis.error.not_completed"
	case errors.Is(err, analysis.ErrBusy):
		code, status, key = "ANALYSIS_BUSY", http.StatusServiceUnavailable, "analysis.error.busy"
	case errors.Is(err, context.DeadlineExceeded):
		code, status, key = "ANALYSIS_TIMEOUT", http.StatusGatewayTimeout, "analysis.error.timeout"
	case err != nil:
		code, status, key = "ANALYSIS_UNAVAILABLE", http.StatusServiceUnavailable, "analysis.error.unavailable"
	}
	handler.logger.InfoContext(request.Context(), "board_analysis_processed", "request_id", requestIDFromContext(request.Context()), "result_code", code, "solver_version", analysis.SolverVersion, "latency_ms", time.Since(startedAt).Milliseconds())
	if err != nil {
		if status == http.StatusServiceUnavailable {
			writer.Header().Set("Retry-After", "2")
		}
		handler.identity.writeError(writer, request, status, code, key, status >= 500)
		return
	}
	writeJSON(writer, http.StatusOK, response)
}
