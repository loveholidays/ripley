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
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewMetricsRecorder_Disabled(t *testing.T) {
	config := MetricsConfig{
		Enabled: false,
		Address: "localhost:9999",
	}

	recorder := NewMetricsRecorder(config, 10)

	if _, ok := recorder.(*noopRecorder); !ok {
		t.Errorf("Expected noopRecorder, got %T", recorder)
	}
}

func TestNewMetricsRecorder_Enabled(t *testing.T) {
	config := MetricsConfig{
		Enabled: true,
		Address: "localhost:18081", // Use different port to avoid conflicts
	}

	recorder := NewMetricsRecorder(config, 10)

	if _, ok := recorder.(*prometheusRecorder); !ok {
		t.Errorf("Expected prometheusRecorder, got %T", recorder)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify metrics endpoint is accessible
	resp, err := http.Get("http://localhost:18081/metrics")
	if err != nil {
		t.Fatalf("Failed to access metrics endpoint: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify it returns Prometheus format
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ripley_") {
		t.Error("Metrics endpoint doesn't contain ripley metrics")
	}
}

func TestNoopRecorder_RecordRequest(t *testing.T) {
	recorder := &noopRecorder{}
	req := &Request{
		Url:    "http://example.com",
		Method: "GET",
	}
	result := &Result{
		Request:    req,
		StatusCode: 200,
		Latency:    100 * time.Millisecond,
	}

	// Should not panic
	recorder.RecordRequest(result)
}

func TestNoopRecorder_StartMonitoring(t *testing.T) {
	recorder := &noopRecorder{}
	requests := make(chan *Request, 1)
	results := make(chan *Result, 1)

	cleanup := recorder.StartMonitoring(requests, results)

	// Should not panic
	cleanup()
}

func TestPrometheusRecorder_RecordRequest(t *testing.T) {
	config := MetricsConfig{
		Enabled: true,
		Address: "localhost:18082",
	}

	recorder := NewMetricsRecorder(config, 10)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	req := &Request{
		Url:    "http://test.example.com/api",
		Method: "GET",
	}

	result := &Result{
		Request:    req,
		StatusCode: 200,
		Latency:    50 * time.Millisecond,
	}

	// Record a request
	recorder.RecordRequest(result)

	// Fetch metrics
	resp, err := http.Get("http://localhost:18082/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	body, _ := io.ReadAll(resp.Body)
	metrics := string(body)

	// Check that request was recorded
	if !strings.Contains(metrics, "ripley_requests_total") {
		t.Error("ripley_requests_total metric not found")
	}

	if !strings.Contains(metrics, "ripley_request_duration_seconds") {
		t.Error("ripley_request_duration_seconds metric not found")
	}

	if !strings.Contains(metrics, "ripley_response_status_total") {
		t.Error("ripley_response_status_total metric not found")
	}

	// Verify host label is used (not full URL)
	if !strings.Contains(metrics, `host="test.example.com"`) {
		t.Error("Expected host label with value 'test.example.com'")
	}
}

func TestPrometheusRecorder_RecordRequestWithError(t *testing.T) {
	config := MetricsConfig{
		Enabled: true,
		Address: "localhost:18083",
	}

	recorder := NewMetricsRecorder(config, 10)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	req := &Request{
		Url:    "http://test.example.com/error",
		Method: "GET",
	}

	result := &Result{
		Request:    req,
		StatusCode: 0,
		ErrorMsg:   "connection refused",
	}

	// Record a failed request
	recorder.RecordRequest(result)

	// Fetch metrics
	resp, err := http.Get("http://localhost:18083/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	body, _ := io.ReadAll(resp.Body)
	metrics := string(body)

	// Check that error was recorded
	if !strings.Contains(metrics, "ripley_errors_total") {
		t.Error("ripley_errors_total metric not found")
	}
}

func TestPrometheusRecorder_StartMonitoring(t *testing.T) {
	config := MetricsConfig{
		Enabled: true,
		Address: "localhost:18084",
	}

	promRecorder := NewMetricsRecorder(config, 10).(*prometheusRecorder)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	requests := make(chan *Request, 10)
	results := make(chan *Result, 10)

	// Add some items to channels
	requests <- &Request{Url: "http://test.com", Method: "GET"}
	requests <- &Request{Url: "http://test.com", Method: "GET"}
	results <- &Result{StatusCode: 200}

	cleanup := promRecorder.StartMonitoring(requests, results)

	// Wait for monitoring to record queue sizes
	time.Sleep(1500 * time.Millisecond)

	// Fetch metrics
	resp, err := http.Get("http://localhost:18084/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	body, _ := io.ReadAll(resp.Body)
	metrics := string(body)

	// Check that queue metrics are present
	if !strings.Contains(metrics, "ripley_request_queue_size") {
		t.Error("ripley_request_queue_size metric not found")
	}

	if !strings.Contains(metrics, "ripley_result_queue_size") {
		t.Error("ripley_result_queue_size metric not found")
	}

	// Cleanup should not panic
	cleanup()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)
}

func TestStartMetricsServer_Disabled(t *testing.T) {
	config := MetricsConfig{
		Enabled: false,
		Address: "localhost:9999",
	}

	errChan := StartMetricsServer(config)

	// Should immediately close channel
	select {
	case err, ok := <-errChan:
		if ok {
			t.Errorf("Expected closed channel, got error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Channel should be closed immediately when disabled")
	}
}

func TestStartMetricsServer_PortInUse(t *testing.T) {
	// Start a server on the test port
	testServer := &http.Server{Addr: "localhost:18085"}
	go func() {
		_ = testServer.ListenAndServe()
	}()
	defer func() {
		_ = testServer.Close()
	}()

	// Give server time to bind
	time.Sleep(100 * time.Millisecond)

	// Try to start metrics server on same port
	config := MetricsConfig{
		Enabled: true,
		Address: "localhost:18085",
	}

	errChan := StartMetricsServer(config)

	// Should receive an error
	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected error for port in use, got nil")
		}
		if !strings.Contains(err.Error(), "address already in use") &&
			!strings.Contains(err.Error(), "bind") {
			t.Errorf("Expected 'address already in use' error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for error from metrics server")
	}
}

func TestSetWorkerPoolSize(t *testing.T) {
	config := MetricsConfig{
		Enabled: true,
		Address: "localhost:18086",
	}

	NewMetricsRecorder(config, 42)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Fetch metrics
	resp, err := http.Get("http://localhost:18086/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	body, _ := io.ReadAll(resp.Body)
	metrics := string(body)

	// Check that worker pool size is set
	if !strings.Contains(metrics, "ripley_worker_pool_size 42") {
		t.Error("Worker pool size not correctly set in metrics")
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "simple http URL",
			url:      "http://example.com/path",
			expected: "example.com",
		},
		{
			name:     "https with port",
			url:      "https://example.com:8080/api/v1/users",
			expected: "example.com:8080",
		},
		{
			name:     "with query params and path",
			url:      "http://api.example.com/users/123?token=abc&id=456",
			expected: "api.example.com",
		},
		{
			name:     "localhost with port",
			url:      "http://localhost:8080/test",
			expected: "localhost:8080",
		},
		{
			name:     "IP address",
			url:      "http://192.168.1.1:9090/metrics",
			expected: "192.168.1.1:9090",
		},
		{
			name:     "invalid URL",
			url:      "not a valid url",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHost(tt.url)
			if result != tt.expected {
				t.Errorf("extractHost(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestMetricsRecorder_MultipleRequests(t *testing.T) {
	config := MetricsConfig{
		Enabled: true,
		Address: "localhost:18087",
	}

	recorder := NewMetricsRecorder(config, 10)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Get initial count
	resp1, err := http.Get("http://localhost:18087/metrics")
	if err != nil {
		t.Fatalf("Failed to get initial metrics: %v", err)
	}
	body1, _ := io.ReadAll(resp1.Body)
	_ = resp1.Body.Close()

	initialMetrics := string(body1)
	initialHasRequests := strings.Contains(initialMetrics, "ripley_requests_total")

	// Record multiple requests
	for i := 0; i < 5; i++ {
		req := &Request{
			Url:    "http://test.example.com/multi",
			Method: "GET",
		}

		result := &Result{
			Request:    req,
			StatusCode: 200,
			Latency:    time.Duration(10*i) * time.Millisecond,
		}

		recorder.RecordRequest(result)
	}

	// Fetch metrics after recording
	resp, err := http.Get("http://localhost:18087/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	body, _ := io.ReadAll(resp.Body)
	metrics := string(body)

	// Check that requests were counted (total should be >= 5)
	if !strings.Contains(metrics, "ripley_requests_total") {
		t.Error("ripley_requests_total metric not found")
	}

	// If we had no requests initially, we should have exactly 5 now
	if !initialHasRequests && !strings.Contains(metrics, "ripley_requests_total 5") {
		t.Error("Expected ripley_requests_total to be 5 for new instance")
	}
}
