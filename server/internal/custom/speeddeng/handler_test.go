package speeddeng

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"xlyra/server/internal/config"
)

func TestMountRegistersSpeedDengRoutes(t *testing.T) {
	router := chi.NewRouter()
	NewHandlerWithService(nil).Mount(router)

	for _, target := range []string{
		"/settings/speed-deng",
		"/settings/speed-deng/start",
		"/settings/speed-deng/stop",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Fatalf("route %s was not mounted", target)
		}
	}
}

func TestStartHandlerReturnsConflictWhenNoEligibleAccountExists(t *testing.T) {
	service := newTestService(&fakeRepository{}, fakeQuotaProvider{})
	handler := NewHandlerWithService(service)
	req := httptest.NewRequest(http.MethodPost, "/settings/speed-deng/start", nil)
	rec := httptest.NewRecorder()
	handler.Start(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body=%s; want 409", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Error.Code != "speed_deng_no_eligible_codex_oauth" {
		t.Fatalf("error code = %q, want speed_deng_no_eligible_codex_oauth", body.Error.Code)
	}
}

func TestStartStopAndGetHandlersReturnState(t *testing.T) {
	repo := &fakeRepository{}
	service := newTestService(repo, fakeQuotaProvider{targets: []QuotaTarget{{SiteID: uuid.New()}}})
	handler := NewHandlerWithService(service)

	startReq := httptest.NewRequest(http.MethodPost, "/settings/speed-deng/start", nil)
	startRec := httptest.NewRecorder()
	handler.Start(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("start status = %d, body=%s", startRec.Code, startRec.Body.String())
	}
	var started Status
	if err := json.NewDecoder(startRec.Body).Decode(&started); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	if !started.Active || started.State != StatusActive || started.SessionID == nil {
		t.Fatalf("start state = %#v, want active session", started)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/settings/speed-deng", nil)
	getRec := httptest.NewRecorder()
	handler.Get(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body=%s", getRec.Code, getRec.Body.String())
	}

	stopReq := httptest.NewRequest(http.MethodPost, "/settings/speed-deng/stop", nil)
	stopRec := httptest.NewRecorder()
	handler.Stop(stopRec, stopReq)
	if stopRec.Code != http.StatusOK {
		t.Fatalf("stop status = %d, body=%s", stopRec.Code, stopRec.Body.String())
	}
	var stopped Status
	if err := json.NewDecoder(stopRec.Body).Decode(&stopped); err != nil {
		t.Fatalf("decode stop response: %v", err)
	}
	if stopped.Active || stopped.State != StatusStopped || stopped.StopReason != StopReasonManual {
		t.Fatalf("stop state = %#v, want manually stopped", stopped)
	}
}

func TestNewHandlerWithServiceUsesDefaultTimeZoneWhenNeeded(t *testing.T) {
	service := NewServiceWithDependencies(&fakeRepository{}, fakeQuotaProvider{}, config.TimeZone{}, nil)
	if NewHandlerWithService(service).service == nil {
		t.Fatal("handler service is nil")
	}
}
