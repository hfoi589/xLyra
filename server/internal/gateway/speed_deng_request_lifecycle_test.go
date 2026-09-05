package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"

	"xlyra/server/internal/auth"
	"xlyra/server/internal/custom/speeddeng"
	routeengine "xlyra/server/internal/router"
	"xlyra/server/internal/store"
)

type speedDengOrderingCapture struct {
	mu      sync.Mutex
	started chan struct{}
}

func (c *speedDengOrderingCapture) BeginRequest(ctx context.Context) (context.Context, bool) {
	c.mu.Lock()
	select {
	case <-c.started:
	default:
		close(c.started)
	}
	c.mu.Unlock()
	return ctx, true
}

func (c *speedDengOrderingCapture) CaptureSuccess(context.Context, speeddeng.CaptureInput) error {
	return nil
}

type speedDengOrderingEndpoint struct {
	captureStarted <-chan struct{}
	decodeSawStart bool
}

func (e *speedDengOrderingEndpoint) DownstreamPath() string { return gatewayEndpointResponses }

func (e *speedDengOrderingEndpoint) RouteEndpointType() string { return "openai-response" }

func (e *speedDengOrderingEndpoint) DecodeRequest(*http.Request) (gatewayRequest, *chatFailure) {
	select {
	case <-e.captureStarted:
		e.decodeSawStart = true
	default:
	}
	return gatewayRequest{}, &chatFailure{
		status:     http.StatusBadRequest,
		code:       "lifecycle_probe",
		message:    "lifecycle probe",
		skipRecord: true,
	}
}

func TestServeEndpointCapturesSpeedDengSessionBeforeDecodingRequest(t *testing.T) {
	capture := &speedDengOrderingCapture{started: make(chan struct{})}
	endpoint := &speedDengOrderingEndpoint{captureStarted: capture.started}
	request := httptest.NewRequest(http.MethodPost, gatewayEndpointResponses, nil)
	request = request.WithContext(auth.WithAPIKey(context.Background(), store.APIKey{ID: uuid.New(), Name: "Wilson"}))
	recorder := httptest.NewRecorder()
	handler := Handler{
		auth:             &auth.Service{},
		router:           &routeengine.Service{},
		db:               &store.Store{},
		speedDengCapture: capture,
	}

	handler.serveEndpoint(recorder, request, endpoint, openAIProtocolResolver{})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want lifecycle probe response", recorder.Code)
	}
	if !endpoint.decodeSawStart {
		t.Fatal("request decoder ran before speed-deng session capture")
	}
}
