package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"
)

func TestRequestAnalyticsRouteIsProtectedAndPrecedesRequestIDRoute(t *testing.T) {
	t.Parallel()

	router := NewRouter(testConfig(), slog.Default(), nil, nil, "test-master-key")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/requests/analytics?view=bar", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertAuthUnavailable(t, rec)
	if rec.Code == http.StatusNotFound {
		t.Fatalf("analytics route was treated as a request log id: body=%s", rec.Body.String())
	}
}
