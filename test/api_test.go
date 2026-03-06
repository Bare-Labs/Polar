package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"polar/internal/api"
	"polar/internal/auth"
	"polar/internal/collector"
	"polar/internal/config"
	"polar/internal/core"
	"polar/internal/providers"
	"polar/internal/storage"
)

func baseConfig() config.Config {
	return config.Config{
		Profile:  "simulator",
		Station:  config.StationConfig{ID: "test", Latitude: 1, Longitude: 1},
		Server:   config.ServerConfig{ListenAddr: ":0", MCPListenAddr: ":0"},
		Storage:  config.StorageConfig{SQLitePath: ":memory:"},
		Auth:     config.AuthConfig{ServiceToken: "dev-token"},
		Features: config.FeatureFlags{EnableForecast: false, EnableMCP: true},
		Polling:  config.PollingConfig{SensorInterval: 1 * time.Second, ForecastInterval: 1 * time.Hour},
		Provider: config.ProviderConfig{OpenMeteoURL: "https://example.com"},
	}
}

func TestHealthEndpoint(t *testing.T) {
	cfg := baseConfig()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	repo := storage.NewRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	svc := core.NewService(cfg, repo, collector.NewSimulatorService(cfg), providers.NewOpenMeteoClient(http.DefaultClient))
	server := api.NewServer(cfg, svc, auth.New(cfg.Auth.ServiceToken))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCapabilitiesRequiresAuth(t *testing.T) {
	cfg := baseConfig()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	repo := storage.NewRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	svc := core.NewService(cfg, repo, collector.NewSimulatorService(cfg), providers.NewOpenMeteoClient(http.DefaultClient))
	server := api.NewServer(cfg, svc, auth.New(cfg.Auth.ServiceToken))

	req := httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestReadyzNotReadyBeforeCollectorRun(t *testing.T) {
	cfg := baseConfig()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	repo := storage.NewRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	svc := core.NewService(cfg, repo, collector.NewSimulatorService(cfg), providers.NewOpenMeteoClient(http.DefaultClient))
	server := api.NewServer(cfg, svc, auth.New(cfg.Auth.ServiceToken))

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response json decode failed: %v", err)
	}
	if payload["status"] != "not_ready" {
		t.Fatalf("expected status not_ready, got %v", payload["status"])
	}
}

func TestReadyzReadyAfterCollectorRun(t *testing.T) {
	cfg := baseConfig()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	repo := storage.NewRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	svc := core.NewService(cfg, repo, collector.NewSimulatorService(cfg), providers.NewOpenMeteoClient(http.DefaultClient))
	if err := svc.PullSensorReadings(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := api.NewServer(cfg, svc, auth.New(cfg.Auth.ServiceToken))

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response json decode failed: %v", err)
	}
	if payload["status"] != "ready" {
		t.Fatalf("expected status ready, got %v", payload["status"])
	}
}
