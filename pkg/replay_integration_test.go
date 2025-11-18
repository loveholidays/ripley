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
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReplayRaceConditionTermination(t *testing.T) {
	// Create a test server that responds slowly to increase chance of race condition
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		// Add small delay to increase race condition likelihood
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create test requests using the test server URL
	testRequests := createTestRequests(server.URL, 50)

	// Save original stdin
	originalStdin := os.Stdin

	// Run multiple iterations to increase chance of reproducing the race condition
	for i := 0; i < 100; i++ {
		t.Run("iteration", func(t *testing.T) {
			// Reset request count
			atomic.StoreInt64(&requestCount, 0)

			// Create a pipe to simulate stdin
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("Failed to create pipe: %v", err)
			}

			// Replace stdin
			os.Stdin = r

			// Write test data to pipe in a goroutine
			go func() {
				defer w.Close()
				_, _ = w.Write([]byte(testRequests))
			}()

			// Capture stdout to suppress output during test
			originalStdout := os.Stdout
			_, captureWriter, _ := os.Pipe()
			os.Stdout = captureWriter

			// Run the replay function with a short phase duration to complete quickly
			// Use high worker count and connections to increase goroutine concurrency
			exitCode := Replay("100ms@10", true, false, 1, false, 20, 100, 0, 0, false, "")

			// Restore stdout
			os.Stdout = originalStdout
			captureWriter.Close()

			// Restore stdin
			os.Stdin = originalStdin
			r.Close()

			// Verify that the function completed successfully
			if exitCode != 0 {
				t.Errorf("Expected exit code 0, got %d", exitCode)
			}

			// Add a small delay to let any lingering goroutines finish
			time.Sleep(50 * time.Millisecond)
		})
	}
}

func TestReplayRaceConditionWithSlowServer(t *testing.T) {
	// Create a server that responds very slowly to maximize race condition exposure
	var wg sync.WaitGroup
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Add(1)
		defer wg.Done()
		// Longer delay to ensure requests are still processing when Replay tries to exit
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	testRequests := createTestRequests(server.URL, 10)

	// Save original stdin/stdout
	originalStdin := os.Stdin
	originalStdout := os.Stdout

	for i := 0; i < 20; i++ {
		t.Run("slow_server_iteration", func(t *testing.T) {
			// Create pipe for stdin
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("Failed to create pipe: %v", err)
			}
			defer r.Close()

			os.Stdin = r

			// Capture stdout
			_, captureWriter, _ := os.Pipe()
			os.Stdout = captureWriter

			// Write test data
			go func() {
				defer w.Close()
				_, _ = w.Write([]byte(testRequests))
			}()

			// Run with very short phase to trigger early completion attempt
			start := time.Now()
			exitCode := Replay("50ms@5", true, false, 1, false, 10, 50, 0, 0, false, "")
			duration := time.Since(start)

			// Restore streams
			os.Stdout = originalStdout
			os.Stdin = originalStdin
			captureWriter.Close()

			if exitCode != 0 {
				t.Errorf("Expected exit code 0, got %d", exitCode)
			}

			// The test should complete in reasonable time despite slow server
			// If it hangs due to race condition, this will fail
			if duration > 5*time.Second {
				t.Errorf("Replay took too long (%v), possible race condition causing hang", duration)
			}
		})
	}

	// Wait for any remaining server requests to complete
	wg.Wait()
}

func TestReplayRaceConditionStressTest(t *testing.T) {
	// Stress test with many concurrent requests and workers
	var requestCounter int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Random delay to simulate real-world variance
		counter := atomic.AddInt64(&requestCounter, 1)
		delay := time.Duration(10+counter%50) * time.Millisecond
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Generate many requests
	testRequests := createTestRequests(server.URL, 200)

	originalStdin := os.Stdin
	originalStdout := os.Stdout

	// Run stress test iterations
	for i := 0; i < 50; i++ {
		t.Run("stress_iteration", func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("Failed to create pipe: %v", err)
			}
			defer r.Close()

			os.Stdin = r
			_, captureWriter, _ := os.Pipe()
			os.Stdout = captureWriter

			go func() {
				defer w.Close()
				_, _ = w.Write([]byte(testRequests))
			}()

			// High concurrency settings to maximize race condition potential
			start := time.Now()
			exitCode := Replay("200ms@20", true, false, 2, false, 50, 200, 0, 0, false, "")
			duration := time.Since(start)

			os.Stdout = originalStdout
			os.Stdin = originalStdin
			captureWriter.Close()

			if exitCode != 0 {
				t.Errorf("Expected exit code 0, got %d", exitCode)
			}

			// Should not hang indefinitely
			if duration > 10*time.Second {
				t.Errorf("Replay took too long (%v), possible deadlock", duration)
			}
		})
	}
}

// Helper function to create test request data
func createTestRequests(serverURL string, count int) string {
	var buffer bytes.Buffer
	baseTime := time.Now()

	for i := 0; i < count; i++ {
		timestamp := baseTime.Add(time.Duration(i) * 100 * time.Millisecond)
		buffer.WriteString(`{"url": "`)
		buffer.WriteString(serverURL)
		buffer.WriteString(`", "method": "GET", "timestamp": "`)
		buffer.WriteString(timestamp.Format(time.RFC3339Nano))
		buffer.WriteString(`"}`)
		buffer.WriteString("\n")
	}

	return buffer.String()
}
