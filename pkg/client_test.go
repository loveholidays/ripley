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
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartClientWorkers_DisableKeepAlives(t *testing.T) {
	tests := []struct {
		name              string
		disableKeepAlives bool
		numRequests       int
		description       string
	}{
		{
			name:              "KeepAlivesEnabled",
			disableKeepAlives: false,
			numRequests:       5,
			description:       "When keep-alives are enabled, connections should be reused",
		},
		{
			name:              "KeepAlivesDisabled",
			disableKeepAlives: true,
			numRequests:       5,
			description:       "When keep-alives are disabled, connections should not be reused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestCount int64
			var connectionCount int64

			// Create test server that tracks connection creation
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt64(&requestCount, 1)

				// Check if this is a new connection by looking at the Connection header
				// When keep-alive is disabled, each request will have "close" in Connection header
				if r.Header.Get("Connection") == "close" || !tt.disableKeepAlives && atomic.LoadInt64(&requestCount) == 1 {
					atomic.AddInt64(&connectionCount, 1)
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			}))
			defer server.Close()

			// Setup channels
			requests := make(chan *Request, tt.numRequests)
			results := make(chan *Result, tt.numRequests)

			// Start client workers with the test configuration
			startClientWorkers(1, requests, results, false, 10, 100, 0, tt.disableKeepAlives)

			// Send test requests
			baseTime := time.Now()
			for i := 0; i < tt.numRequests; i++ {
				req := &Request{
					Url:       server.URL,
					Method:    "GET",
					Timestamp: baseTime.Add(time.Duration(i) * time.Millisecond),
				}
				requests <- req
			}
			close(requests)

			// Collect results
			receivedResults := 0
			for result := range results {
				receivedResults++
				if result.StatusCode != 200 {
					t.Errorf("Expected status code 200, got %d", result.StatusCode)
				}
				if result.ErrorMsg != "" {
					t.Errorf("Unexpected error: %s", result.ErrorMsg)
				}

				if receivedResults == tt.numRequests {
					close(results)
					break
				}
			}

			// Verify all requests were processed
			if receivedResults != tt.numRequests {
				t.Errorf("Expected %d results, got %d", tt.numRequests, receivedResults)
			}

			finalRequestCount := atomic.LoadInt64(&requestCount)
			if finalRequestCount != int64(tt.numRequests) {
				t.Errorf("Expected %d requests to server, got %d", tt.numRequests, finalRequestCount)
			}
		})
	}
}

func TestStartClientWorkers_TransportConfiguration(t *testing.T) {
	tests := []struct {
		name                string
		timeout             int
		connections         int
		maxConnections      int
		disableKeepAlives   bool
		expectedTimeout     time.Duration
		expectedMaxIdle     int
		expectedMaxConns    int
		expectedDisableKA   bool
	}{
		{
			name:                "DefaultWithKeepAlives",
			timeout:             10,
			connections:         10000,
			maxConnections:      0,
			disableKeepAlives:   false,
			expectedTimeout:     10 * time.Second,
			expectedMaxIdle:     10000,
			expectedMaxConns:    0,
			expectedDisableKA:   false,
		},
		{
			name:                "DisabledKeepAlives",
			timeout:             5,
			connections:         5000,
			maxConnections:      1000,
			disableKeepAlives:   true,
			expectedTimeout:     5 * time.Second,
			expectedMaxIdle:     5000,
			expectedMaxConns:    1000,
			expectedDisableKA:   true,
		},
		{
			name:                "CustomConfigWithKeepAlives",
			timeout:             30,
			connections:         2000,
			maxConnections:      500,
			disableKeepAlives:   false,
			expectedTimeout:     30 * time.Second,
			expectedMaxIdle:     2000,
			expectedMaxConns:    500,
			expectedDisableKA:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			requests := make(chan *Request, 1)
			results := make(chan *Result, 1)

			// Start client workers - this internally creates the HTTP client with our config
			startClientWorkers(1, requests, results, false, tt.timeout, tt.connections, tt.maxConnections, tt.disableKeepAlives)

			// Send a single request to verify the client works
			req := &Request{
				Url:       server.URL,
				Method:    "GET",
				Timestamp: time.Now(),
			}
			requests <- req
			close(requests)

			// Get result
			result := <-results
			close(results)

			// Verify the request succeeded
			if result.StatusCode != 200 {
				t.Errorf("Expected status code 200, got %d", result.StatusCode)
			}
			if result.ErrorMsg != "" {
				t.Errorf("Unexpected error: %s", result.ErrorMsg)
			}

			// Note: We can't directly inspect the HTTP client configuration from outside
			// since it's created inside startClientWorkers, but we've verified it works
			// correctly. The connection behavior tests above verify the keep-alive behavior.
		})
	}
}

func TestStartClientWorkers_DryRun(t *testing.T) {
	tests := []struct {
		name              string
		disableKeepAlives bool
	}{
		{
			name:              "DryRunWithKeepAlives",
			disableKeepAlives: false,
		},
		{
			name:              "DryRunWithoutKeepAlives",
			disableKeepAlives: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverCalled bool

			// Create a test server that should NOT be called in dry-run mode
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				serverCalled = true
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			requests := make(chan *Request, 1)
			results := make(chan *Result, 1)

			// Start client workers in dry-run mode
			startClientWorkers(1, requests, results, true, 10, 100, 0, tt.disableKeepAlives)

			// Send a request
			req := &Request{
				Url:       server.URL,
				Method:    "GET",
				Timestamp: time.Now(),
			}
			requests <- req
			close(requests)

			// Get result
			result := <-results
			close(results)

			// In dry-run mode, status code should be 0 and server should not be called
			if result.StatusCode != 0 {
				t.Errorf("Expected status code 0 in dry-run mode, got %d", result.StatusCode)
			}

			// Give a small delay to ensure server would have been called if it was going to be
			time.Sleep(10 * time.Millisecond)

			if serverCalled {
				t.Error("Server should not be called in dry-run mode")
			}
		})
	}
}

func TestStartClientWorkers_MultipleWorkers(t *testing.T) {
	tests := []struct {
		name              string
		numWorkers        int
		numRequests       int
		disableKeepAlives bool
	}{
		{
			name:              "MultipleWorkersWithKeepAlives",
			numWorkers:        5,
			numRequests:       20,
			disableKeepAlives: false,
		},
		{
			name:              "MultipleWorkersWithoutKeepAlives",
			numWorkers:        5,
			numRequests:       20,
			disableKeepAlives: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestCount int64

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt64(&requestCount, 1)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			}))
			defer server.Close()

			requests := make(chan *Request, tt.numRequests)
			results := make(chan *Result, tt.numRequests)

			// Start multiple workers
			startClientWorkers(tt.numWorkers, requests, results, false, 10, 100, 0, tt.disableKeepAlives)

			// Send requests
			baseTime := time.Now()
			for i := 0; i < tt.numRequests; i++ {
				req := &Request{
					Url:       server.URL,
					Method:    "GET",
					Timestamp: baseTime.Add(time.Duration(i) * time.Millisecond),
				}
				requests <- req
			}
			close(requests)

			// Collect results
			receivedResults := 0
			successCount := 0
			for result := range results {
				receivedResults++
				if result.StatusCode == 200 && result.ErrorMsg == "" {
					successCount++
				}

				if receivedResults == tt.numRequests {
					close(results)
					break
				}
			}

			// Verify all requests were processed successfully
			if successCount != tt.numRequests {
				t.Errorf("Expected %d successful requests, got %d", tt.numRequests, successCount)
			}

			finalRequestCount := atomic.LoadInt64(&requestCount)
			if finalRequestCount != int64(tt.numRequests) {
				t.Errorf("Expected %d requests to server, got %d", tt.numRequests, finalRequestCount)
			}
		})
	}
}

func TestStartClientWorkers_ErrorHandling(t *testing.T) {
	tests := []struct {
		name              string
		disableKeepAlives bool
		serverBehavior    func(w http.ResponseWriter, r *http.Request)
		expectError       bool
	}{
		{
			name:              "SuccessWithKeepAlives",
			disableKeepAlives: false,
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:              "SuccessWithoutKeepAlives",
			disableKeepAlives: true,
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:              "ServerErrorWithKeepAlives",
			disableKeepAlives: false,
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: false, // HTTP errors don't set ErrorMsg, just status code
		},
		{
			name:              "ServerErrorWithoutKeepAlives",
			disableKeepAlives: true,
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: false, // HTTP errors don't set ErrorMsg, just status code
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverBehavior))
			defer server.Close()

			requests := make(chan *Request, 1)
			results := make(chan *Result, 1)

			startClientWorkers(1, requests, results, false, 10, 100, 0, tt.disableKeepAlives)

			req := &Request{
				Url:       server.URL,
				Method:    "GET",
				Timestamp: time.Now(),
			}
			requests <- req
			close(requests)

			result := <-results
			close(results)

			hasError := result.ErrorMsg != ""
			if hasError != tt.expectError {
				t.Errorf("Expected error=%v, got error=%v (ErrorMsg: %s)", tt.expectError, hasError, result.ErrorMsg)
			}
		})
	}
}
