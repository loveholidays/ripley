/*
ripley
Copyright (C) 2021  loveholidays

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package ripley

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Request duration histogram
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ripley_request_duration_seconds",
			Help:    "HTTP request latencies in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"url"},
	)

	// Response status code counter
	responseStatus = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ripley_response_status_total",
			Help: "Total number of HTTP responses by status code",
		},
		[]string{"status_code", "url"},
	)

	// Total requests counter
	requestsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ripley_requests_total",
			Help: "Total number of HTTP requests sent",
		},
	)

	// Errors counter
	errorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ripley_errors_total",
			Help: "Total number of errors",
		},
		[]string{"url"},
	)

	// Pacer phase gauge
	pacerPhase = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ripley_pacer_phase",
			Help: "Current pacer phase rate multiplier",
		},
		[]string{"phase"},
	)

	// Worker pool size gauge
	workerPoolSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ripley_worker_pool_size",
			Help: "Number of worker goroutines",
		},
	)

	// Request queue size gauge
	requestQueueSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ripley_request_queue_size",
			Help: "Current size of the request queue",
		},
	)

	// Result queue size gauge
	resultQueueSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ripley_result_queue_size",
			Help: "Current size of the result queue",
		},
	)
)

// MetricsConfig holds metrics server configuration
type MetricsConfig struct {
	Enabled bool
	Address string
}

// MetricsRecorder interface for recording metrics
type MetricsRecorder interface {
	RecordRequest(result *Result)
	StartMonitoring(requests chan *Request, results chan *Result) func()
}

// prometheusRecorder implements MetricsRecorder with actual Prometheus metrics
type prometheusRecorder struct {
	stopMonitoring chan bool
}

// noopRecorder implements MetricsRecorder with no-op implementations
type noopRecorder struct{}

// NewMetricsRecorder creates a metrics recorder based on configuration
func NewMetricsRecorder(config MetricsConfig, numWorkers int) MetricsRecorder {
	if config.Enabled {
		errChan := StartMetricsServer(config)

		// Monitor for server errors in background
		go func() {
			if err := <-errChan; err != nil {
				log.Printf("WARNING: Metrics server unavailable, continuing without metrics: %v", err)
			}
		}()

		SetWorkerPoolSize(numWorkers)
		return &prometheusRecorder{stopMonitoring: make(chan bool)}
	}
	return &noopRecorder{}
}

func (p *prometheusRecorder) RecordRequest(result *Result) {
	RecordRequest(result)
}

func (p *prometheusRecorder) StartMonitoring(requests chan *Request, results chan *Result) func() {
	go MonitorQueueSizes(requests, results, p.stopMonitoring)
	return func() {
		p.stopMonitoring <- true
	}
}

func (n *noopRecorder) RecordRequest(result *Result) {}

func (n *noopRecorder) StartMonitoring(requests chan *Request, results chan *Result) func() {
	return func() {} // Return no-op cleanup function
}

func init() {
	// Register metrics with Prometheus's default registry
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(responseStatus)
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(errorsTotal)
	prometheus.MustRegister(pacerPhase)
	prometheus.MustRegister(workerPoolSize)
	prometheus.MustRegister(requestQueueSize)
	prometheus.MustRegister(resultQueueSize)
}

// StartMetricsServer starts the Prometheus metrics HTTP server
// Returns an error channel that will receive any server startup or runtime errors
func StartMetricsServer(config MetricsConfig) <-chan error {
	errChan := make(chan error, 1)

	if !config.Enabled {
		close(errChan)
		return errChan
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    config.Address,
		Handler: mux,
	}

	go func() {
		log.Printf("Starting metrics server on %s", config.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server failed: %v", err)
			errChan <- err
		}
		close(errChan)
	}()

	return errChan
}

// RecordRequest records metrics for a completed HTTP request
func RecordRequest(result *Result) {
	requestsTotal.Inc()

	if result.ErrorMsg != "" {
		errorsTotal.WithLabelValues(result.Request.Url).Inc()
	} else {
		requestDuration.WithLabelValues(result.Request.Url).Observe(result.Latency.Seconds())
		responseStatus.WithLabelValues(http.StatusText(result.StatusCode), result.Request.Url).Inc()
	}
}

// SetWorkerPoolSize sets the worker pool size metric
func SetWorkerPoolSize(size int) {
	workerPoolSize.Set(float64(size))
}

// SetPacerPhase sets the current pacer phase
func SetPacerPhase(phase string, rate float64) {
	pacerPhase.WithLabelValues(phase).Set(rate)
}

// MonitorQueueSizes monitors the request and result queue sizes
func MonitorQueueSizes(requests chan *Request, results chan *Result, done chan bool) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			requestQueueSize.Set(float64(len(requests)))
			resultQueueSize.Set(float64(len(results)))
		}
	}
}
