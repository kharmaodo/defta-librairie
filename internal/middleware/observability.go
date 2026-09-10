package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Route labels come from ServeMux patterns, never from raw request URLs.
type HTTPMetric struct {
	Route              string  `json:"route"`
	Status             int     `json:"status"`
	Requests           uint64  `json:"requests"`
	Bytes              uint64  `json:"bytes"`
	DurationSeconds    float64 `json:"durationSeconds"`
	MaxDurationSeconds float64 `json:"maxDurationSeconds"`
}
type HTTPSnapshot struct {
	StartedAt     string       `json:"startedAt"`
	UptimeSeconds float64      `json:"uptimeSeconds"`
	InFlight      int64        `json:"inFlight"`
	Completed     uint64       `json:"completed"`
	Routes        []HTTPMetric `json:"routes"`
}
type metricKey struct {
	route  string
	status int
}
type HTTPObservability struct {
	mu        sync.Mutex
	started   time.Time
	inFlight  int64
	completed uint64
	metrics   map[metricKey]HTTPMetric
	logger    *slog.Logger
}

func NewHTTPObservability(logger *slog.Logger) *HTTPObservability {
	if logger == nil {
		logger = slog.Default()
	}
	return &HTTPObservability{started: time.Now(), metrics: make(map[metricKey]HTTPMetric), logger: logger}
}

type observedWriter struct {
	http.ResponseWriter
	status int
	bytes  uint64
}

func (w *observedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *observedWriter) WriteHeader(status int) {
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status != 0 {
		return
	}
	w.ResponseWriter.WriteHeader(status)
	w.status = status
}
func (w *observedWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += uint64(n)
	return n, err
}
func (o *HTTPObservability) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		o.mu.Lock()
		o.inFlight++
		o.mu.Unlock()
		observed := &observedWriter{ResponseWriter: w}
		completed := false
		defer func() {
			status := observed.status
			if status == 0 {
				status = 200
				if !completed {
					status = 500
				}
			}
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			elapsed := time.Since(start).Seconds()
			key := metricKey{route, status}
			o.mu.Lock()
			o.inFlight--
			o.completed++
			if _, ok := o.metrics[key]; !ok && len(o.metrics) >= 1024 {
				key = metricKey{"overflow", 0}
			}
			m := o.metrics[key]
			m.Route = key.route
			m.Status = key.status
			m.Requests++
			m.Bytes += observed.bytes
			m.DurationSeconds += elapsed
			if elapsed > m.MaxDurationSeconds {
				m.MaxDurationSeconds = elapsed
			}
			o.metrics[key] = m
			o.mu.Unlock()
			level := slog.LevelInfo
			if status >= 500 || !completed {
				level = slog.LevelError
			}
			o.logger.Log(context.Background(), level, "http_request", "request_id", w.Header().Get("X-Request-ID"), "route", route, "status", status, "bytes", observed.bytes, "duration_ms", elapsed*1000, "aborted", !completed)
		}()
		next.ServeHTTP(observed, r)
		completed = true
	})
}
func (o *HTTPObservability) Snapshot() HTTPSnapshot {
	o.mu.Lock()
	defer o.mu.Unlock()
	s := HTTPSnapshot{StartedAt: o.started.UTC().Format(time.RFC3339Nano), UptimeSeconds: time.Since(o.started).Seconds(), InFlight: o.inFlight, Completed: o.completed, Routes: make([]HTTPMetric, 0, len(o.metrics))}
	for _, m := range o.metrics {
		s.Routes = append(s.Routes, m)
	}
	sort.Slice(s.Routes, func(i, j int) bool {
		if s.Routes[i].Route == s.Routes[j].Route {
			return s.Routes[i].Status < s.Routes[j].Status
		}
		return s.Routes[i].Route < s.Routes[j].Route
	})
	return s
}

// Metrics must be registered behind authentication and the root role guard.
func (o *HTTPObservability) Metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(o.Snapshot())
}
