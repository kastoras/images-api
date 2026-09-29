package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics records per-request counters/histograms against reg. It labels by
// route template (mux.CurrentRoute), not raw path — several routes carry
// path params (/jobs/{id}, /assets/{tenant}/{hash}, /a/{consumer}/{tenant}/{hash}/render),
// and labeling by raw path would let each distinct ID/hash create its own
// time series.
func Metrics(reg *prometheus.Registry) mux.MiddlewareFunc {
	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests, by method, route and status.",
		},
		[]string{"method", "route", "status"},
	)
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request duration in seconds, by method and route.",
		},
		[]string{"method", "route"},
	)
	reg.MustRegister(requestsTotal, requestDuration)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, r)

			route := r.URL.Path
			if current := mux.CurrentRoute(r); current != nil {
				if tmpl, err := current.GetPathTemplate(); err == nil {
					route = tmpl
				}
			}

			requestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
			requestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
		})
	}
}
