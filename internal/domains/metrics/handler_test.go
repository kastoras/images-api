package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/server"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsEndpoint(t *testing.T) {
	reg := prometheus.NewRegistry()
	g := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_gauge", Help: "test"})
	reg.MustRegister(g)
	g.Set(3)

	router := mux.NewRouter()
	Register(router, &server.APIServer{Metrics: reg})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "test_gauge 3") {
		t.Fatalf("body missing test_gauge:\n%s", rec.Body.String())
	}
}

func TestMetricsEndpoint_GETOnly(t *testing.T) {
	router := mux.NewRouter()
	Register(router, &server.APIServer{Metrics: prometheus.NewRegistry()})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/metrics", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
