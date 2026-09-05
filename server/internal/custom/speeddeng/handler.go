package speeddeng

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"xlyra/server/internal/config"
	"xlyra/server/internal/httpx"
	"xlyra/server/internal/store"
)

const (
	speedDengRoute      = "/settings/speed-deng"
	speedDengStartRoute = "/settings/speed-deng/start"
	speedDengStopRoute  = "/settings/speed-deng/stop"
)

type Handler struct {
	service *Service
}

func NewHandler(db *store.Store, provider QuotaProvider, timeZones ...config.TimeZone) Handler {
	zone := config.TimeZoneOrDefault(timeZones...)
	return Handler{service: NewService(db, provider, zone, slog.Default())}
}

func NewHandlerWithService(service *Service) Handler { return Handler{service: service} }

func (h Handler) Mount(r chi.Router) {
	r.Get(speedDengRoute, h.Get)
	r.Post(speedDengStartRoute, h.Start)
	r.Post(speedDengStopRoute, h.Stop)
}

func (h Handler) Get(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		httpx.Error(w, r, http.StatusServiceUnavailable, "speed_deng_unavailable", "speed-deng service is not available")
		return
	}
	status, err := h.service.Status(r.Context())
	if err != nil {
		httpx.Error(w, r, http.StatusServiceUnavailable, "speed_deng_unavailable", "speed-deng service is not available")
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}

func (h Handler) Start(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		httpx.Error(w, r, http.StatusServiceUnavailable, "speed_deng_unavailable", "speed-deng service is not available")
		return
	}
	session, err := h.service.Start(r.Context(), time.Now())
	if err != nil {
		switch {
		case errors.Is(err, ErrNoEligibleAccounts):
			httpx.Error(w, r, http.StatusConflict, "speed_deng_no_eligible_codex_oauth", "no connected and enabled Codex OAuth account is available")
		case errors.Is(err, ErrServiceUnavailable):
			httpx.Error(w, r, http.StatusServiceUnavailable, "speed_deng_unavailable", "speed-deng service is not available")
		default:
			httpx.Error(w, r, http.StatusInternalServerError, "speed_deng_start_failed", "failed to start speed-deng")
		}
		return
	}
	status, err := h.service.Status(r.Context())
	if err != nil {
		httpx.JSON(w, http.StatusOK, h.service.statusForSession(r.Context(), session))
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}

func (h Handler) Stop(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		httpx.Error(w, r, http.StatusServiceUnavailable, "speed_deng_unavailable", "speed-deng service is not available")
		return
	}
	stopped, err := h.service.Stop(r.Context(), time.Now(), StopReasonManual)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "speed_deng_stop_failed", "failed to stop speed-deng")
		return
	}
	status, err := h.service.Status(r.Context())
	if err != nil {
		httpx.JSON(w, http.StatusOK, h.service.statusForSession(r.Context(), stopped))
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}
