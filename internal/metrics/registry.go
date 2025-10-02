package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)


// Handler returns an HTTP handler for the metrics endpoint
func Handler() http.Handler {
	return promhttp.Handler()
}

// HandlerFor returns an HTTP handler for a specific registry
func HandlerFor(registry *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}