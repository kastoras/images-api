package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func newMetricsRouter(reg *prometheus.Registry) *mux.Router {
	r := mux.NewRouter()
	r.Use(Metrics(reg))
	r.HandleFunc("/jobs/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}).Methods("GET")
	return r
}

func TestMetrics_LabelsByRouteTemplate(t *testing.T) {
	reg := prometheus.NewRegistry()
	r := newMetricsRouter(reg)

	for _, id := range []string{"a", "b", "c"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/jobs/"+id, nil))
	}

	// Three distinct IDs must collapse into one series: requests + duration.
	if got := testutil.CollectAndCount(reg, "http_requests_total"); got != 1 {
		t.Fatalf("http_requests_total series = %d, want 1", got)
	}
	if got := testutil.CollectAndCount(reg, "http_request_duration_seconds"); got != 1 {
		t.Fatalf("http_request_duration_seconds series = %d, want 1", got)
	}
}

func TestMetrics_RecordsStatus(t *testing.T) {
	reg := prometheus.NewRegistry()
	r := newMetricsRouter(reg)
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/jobs/x", nil))

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "http_requests_total" {
			continue
		}
		labels := map[string]string{}
		for _, l := range mf.GetMetric()[0].GetLabel() {
			labels[l.GetName()] = l.GetValue()
		}
		if labels["status"] != "202" || labels["route"] != "/jobs/{id}" || labels["method"] != "GET" {
			t.Fatalf("unexpected labels: %v", labels)
		}
		return
	}
	t.Fatal("http_requests_total not found")
}

func TestMetrics_SameRegistryTwiceDoesNotPanic(t *testing.T) {
	reg := prometheus.NewRegistry()
	newMetricsRouter(reg)
	newMetricsRouter(reg)
}
