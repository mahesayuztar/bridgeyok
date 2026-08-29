package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicRouterDoesNotExposeEcho(t *testing.T) {
	t.Parallel()

	handler, _ := testRouter(readinessFunc(func(context.Context) error { return nil }))
	request := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
