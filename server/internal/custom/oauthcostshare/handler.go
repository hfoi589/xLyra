package oauthcostshare

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"xlyra/server/internal/config"
	"xlyra/server/internal/httpx"
	"xlyra/server/internal/store"
)

const (
	costShareRoute         = "/requests/oauth-cost-share"
	costShareSettingsRoute = "/settings/oauth-cost-share"
)

type Handler struct {
	service  *Service
	confFile *config.ConfigFile
}

func NewHandler(db *store.Store, confFile *config.ConfigFile, timeZones ...config.TimeZone) Handler {
	return Handler{
		service:  NewService(db, confFile, timeZones...),
		confFile: confFile,
	}
}

func NewHandlerWithService(service *Service, confFile *config.ConfigFile) Handler {
	return Handler{service: service, confFile: confFile}
}

func (h Handler) Mount(r chi.Router) {
	r.Get(costShareRoute, h.GetCostShare)
	r.Get(costShareSettingsRoute, h.GetConfig)
	r.Put(costShareSettingsRoute, h.UpdateConfig)
}

func (h Handler) GetConfig(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, ReadConfig(h.confFile))
}

func (h Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var cfg Config
	if err := httpx.DecodeJSONBody(r, &cfg); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if err := ValidateConfig(cfg); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_oauth_cost_share_config", err.Error())
		return
	}
	if h.confFile == nil {
		httpx.Error(w, r, http.StatusServiceUnavailable, "config_unavailable", "config persistence is not available")
		return
	}
	if err := SaveConfig(h.confFile, cfg); err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "config_write_error", "failed to save OAuth cost share config")
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}

func (h Handler) GetCostShare(w http.ResponseWriter, r *http.Request) {
	query, ok := parseCostShareQuery(w, r)
	if !ok {
		return
	}
	if h.service == nil {
		httpx.Error(w, r, http.StatusServiceUnavailable, "oauth_cost_share_unavailable", "OAuth cost share service is not available")
		return
	}
	result, err := h.service.CostShare(r.Context(), query, time.Now())
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "oauth_cost_share_failed", "failed to load OAuth cost share")
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func parseCostShareQuery(w http.ResponseWriter, r *http.Request) (CostShareQuery, bool) {
	values := r.URL.Query()["site_id"]
	if len(values) != 1 || strings.TrimSpace(values[0]) == "" {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_site_id", "site_id must contain exactly one UUID")
		return CostShareQuery{}, false
	}
	siteID, err := uuid.Parse(strings.TrimSpace(values[0]))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_site_id", "site_id must contain exactly one UUID")
		return CostShareQuery{}, false
	}
	createdFrom, ok := parseCostShareTime(w, r, "created_from")
	if !ok {
		return CostShareQuery{}, false
	}
	createdTo, ok := parseCostShareTime(w, r, "created_to")
	if !ok {
		return CostShareQuery{}, false
	}
	return CostShareQuery{SiteID: siteID, CreatedFrom: createdFrom, CreatedTo: createdTo}, true
}

func parseCostShareTime(w http.ResponseWriter, r *http.Request, key string) (*time.Time, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, true
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_"+key, key+" must be an RFC3339 timestamp")
		return nil, false
	}
	return &value, true
}
