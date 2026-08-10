package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	namespace = "haze"

	HTTPRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"service", "method", "path", "status"},
	)

	HTTPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"service", "method", "path"},
	)

	WSConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "ws_connections",
			Help:      "Current number of active WebSocket connections.",
		},
		[]string{"service"},
	)

	WSMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "ws_messages_total",
			Help:      "Total number of WebSocket messages processed.",
		},
		[]string{"service", "direction"},
	)
)

func init() {
	prometheus.MustRegister(HTTPRequests, HTTPDuration, WSConnections, WSMessagesTotal)
}

// Handler возвращает promhttp.Handler для /metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}

// StatusFromCode превращает код ответа в метку prometheus (2xx, 3xx, ...).
func StatusFromCode(code int) string {
	return statusClass(code)
}

func statusClass(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	default:
		return "other"
	}
}
