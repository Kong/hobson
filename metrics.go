package main

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ServiceMonitorRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hobson_service_monitor_running",
			Help: "The number of monitors hobson is running for a service (should normally be one)",
		},
		[]string{"service"},
	)

	ServiceLastUpdate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hobson_service_last_updated_timestamp",
			Help: "Timestamp of the service node lists last updated",
		},
		[]string{"service"},
	)

	ServiceFetchFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hobson_service_fetch_failure_count",
			Help: "Counts of service fetch failures from consul",
		},
		[]string{"service"},
	)

	RecordLastServed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hobson_record_last_served_timstamp",
			Help: "Timestamp of the service with specified record returned",
		},
		[]string{"domain", "record"},
	)
)

type MetricsServerConfig struct {
	ListenAddress string
}

type MetricsServer struct {
	listenAddress string
	http          *http.Server
}

func NewMeticsServer(c *MetricsServerConfig) *MetricsServer {
	prom := promhttp.Handler()

	http.HandleFunc("/metrics", prom.ServeHTTP)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		<head><title>Hobson Metrics</title></head>
		<body>
		<h1>Hobson Metrics</h1>
		<img src="https://upload.wikimedia.org/wikipedia/commons/9/9d/ThomasHobson.jpg"/>
		<p><a href="/metrics">Metrics</a></p>
		</body>
		</html>`))
	})
	httpServer := &http.Server{
		Addr: c.ListenAddress,
	}

	return &MetricsServer{
		http: httpServer,
	}
}

func (m *MetricsServer) RegisterMetrics() {
	prometheus.MustRegister(
		ServiceMonitorRunning,
		ServiceLastUpdate,
		ServiceFetchFailures,
		RecordLastServed)
}

func (m *MetricsServer) ListenAndServe() error {
	return m.http.ListenAndServe()
}

func (m *MetricsServer) ShutdownContext(ctx context.Context) error {
	return m.http.Shutdown(ctx)
}
