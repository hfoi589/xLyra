package oauthcostshare

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"xlyra/server/internal/config"
)

func TestUpdateConfigPersistsAndReturnsOAuthCostShareConfig(t *testing.T) {
	t.Parallel()

	confFile, err := config.LoadConfigFile(t.TempDir())
	if err != nil {
		t.Fatalf("load config file: %v", err)
	}
	handler := NewHandlerWithService(nil, confFile)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/oauth-cost-share", strings.NewReader(`{
		"plus":{"single_quota":100,"reset_count":1,"account_fee":20},
		"pro_lite":{"single_quota":200,"reset_count":2,"account_fee":30},
		"pro":{"single_quota":300,"reset_count":3,"account_fee":40}
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.UpdateConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var got Config
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	want := Config{
		Plus:    PlanConfig{SingleQuota: 100, ResetCount: 1, AccountFee: 20},
		ProLite: PlanConfig{SingleQuota: 200, ResetCount: 2, AccountFee: 30},
		Pro:     PlanConfig{SingleQuota: 300, ResetCount: 3, AccountFee: 40},
	}
	if got != want || ReadConfig(confFile) != want {
		t.Fatalf("config response/persistence = %#v/%#v, want %#v", got, ReadConfig(confFile), want)
	}
}

func TestUpdateConfigRejectsNegativePlanValues(t *testing.T) {
	t.Parallel()

	handler := NewHandlerWithService(nil, nil)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/oauth-cost-share", strings.NewReader(`{
		"plus":{"single_quota":-1,"reset_count":0,"account_fee":1}
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.UpdateConfig(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "invalid_oauth_cost_share_config" {
		t.Fatalf("error code = %q, want invalid_oauth_cost_share_config", body.Error.Code)
	}
}

func TestCostShareHandlerRequiresExactlyOneSite(t *testing.T) {
	t.Parallel()

	handler := NewHandlerWithService(nil, nil)
	for name, target := range map[string]string{
		"missing":  "/api/v1/requests/oauth-cost-share",
		"multiple": "/api/v1/requests/oauth-cost-share?site_id=" + uuid.NewString() + "&site_id=" + uuid.NewString(),
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()
			handler.GetCostShare(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCostShareHandlerReturnsServicePayload(t *testing.T) {
	t.Parallel()

	siteID := uuid.New()
	service := NewServiceWithSource(fakeSource{
		site: OAuthSite{ID: siteID, Name: "Codex", PlanType: "plus", IsOAuth: true},
		rows: []UsageRow{{ModelKey: "gpt-5", APIKeyName: "Wilson", Cost: 40, RequestCount: 2}},
	}, nil, config.LoadTimeZone("UTC"))
	handler := NewHandlerWithService(service, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/requests/oauth-cost-share?site_id="+siteID.String(), nil)
	rec := httptest.NewRecorder()
	handler.GetCostShare(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body CostShareResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Meta.Currency != "USD" || body.Data.SiteID != siteID.String() {
		t.Fatalf("response = %#v, want USD and site id", body)
	}
}

func TestMountRegistersCostShareAndSettingsRoutes(t *testing.T) {
	t.Parallel()

	confFile, err := config.LoadConfigFile(t.TempDir())
	if err != nil {
		t.Fatalf("load config file: %v", err)
	}
	router := chi.NewRouter()
	NewHandlerWithService(nil, confFile).Mount(router)

	for _, target := range []string{
		"/settings/oauth-cost-share",
		"/requests/oauth-cost-share",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Fatalf("route %s was not mounted", target)
		}
	}
}

var _ UsageSource = fakeSource{}
