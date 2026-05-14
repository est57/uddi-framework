package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/uddi-protocol/uddi/api/internal/response"
)

type Metrics struct {
	startedAt        time.Time
	requestsTotal    atomic.Uint64
	responsesTotal   atomic.Uint64
	errorsTotal      atomic.Uint64
	inFlightRequests atomic.Int64
	latencyTotalMS   atomic.Uint64
}

func NewMetrics() *Metrics {
	return &Metrics{startedAt: time.Now().UTC()}
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		m.requestsTotal.Add(1)
		m.inFlightRequests.Add(1)
		defer m.inFlightRequests.Add(-1)

		next.ServeHTTP(recorder, r)

		duration := time.Since(start)
		m.responsesTotal.Add(1)
		m.latencyTotalMS.Add(uint64(duration.Milliseconds()))
		if recorder.status >= 500 {
			m.errorsTotal.Add(1)
		}

		slog.Info(
			"http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"request_id", requestID(r),
			"user_agent", r.UserAgent(),
		)
	})
}

func (m *Metrics) Handler(w http.ResponseWriter, _ *http.Request) {
	latencyTotal := m.latencyTotalMS.Load()
	responsesTotal := m.responsesTotal.Load()
	averageLatency := float64(0)
	if responsesTotal > 0 {
		averageLatency = float64(latencyTotal) / float64(responsesTotal)
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"startedAt":          m.startedAt.Format(time.RFC3339),
		"uptimeSeconds":      int64(time.Since(m.startedAt).Seconds()),
		"requestsTotal":      m.requestsTotal.Load(),
		"responsesTotal":     responsesTotal,
		"errorsTotal":        m.errorsTotal.Load(),
		"inFlightRequests":   m.inFlightRequests.Load(),
		"latencyTotalMs":     latencyTotal,
		"latencyAverageMs":   averageLatency,
		"metricsContentType": "application/json",
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func requestID(r *http.Request) string {
	if requestID := chimiddleware.GetReqID(r.Context()); requestID != "" {
		return requestID
	}
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		return requestID
	}
	if requestID := r.Header.Get("X-Correlation-ID"); requestID != "" {
		return requestID
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
