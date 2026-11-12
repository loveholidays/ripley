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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// outcome is the part of a Result that the tests compare.
type outcome struct {
	status       int
	disconnected bool
	failed       bool
}

func outcomeOf(res *Result) outcome {
	return outcome{status: res.StatusCode, disconnected: res.Disconnected, failed: res.ErrorMsg != ""}
}

// serve starts a test server for h and returns its URL.
func serve(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}

// sseEvents streams an event every 10ms until the client leaves.
func sseEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := fmt.Fprint(w, "data: event\n\n"); err != nil {
			return
		}
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func writeOK(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("ok"))
}

func neverRespond(w http.ResponseWriter, r *http.Request) {
	<-r.Context().Done()
}

func truncatedBody(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Length", "100")
	_, _ = w.Write([]byte("truncated"))
}

func testClient(t testing.TB, timeout time.Duration) *http.Client {
	t.Helper()
	client := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{MaxIdleConnsPerHost: 10},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	t.Cleanup(client.CloseIdleConnections)
	return client
}

func runRequest(client *http.Client, req *Request) *Result {
	results := make(chan *Result, 1)
	executeRequest(client, req, time.Now(), results)
	return <-results
}

// get builds a GET request. A zero disconnectAfter makes an ordinary request.
func get(url string, disconnectAfter time.Duration) *Request {
	return &Request{Method: "GET", Url: url, DisconnectAfter: Duration(disconnectAfter)}
}

func TestSessionOutcome(t *testing.T) {
	tests := []struct {
		name            string
		handler         http.HandlerFunc
		disconnectAfter time.Duration
		want            outcome
		minLatency      time.Duration
	}{
		{"deadline is not an error", sseEvents, 200 * time.Millisecond, outcome{status: 200, disconnected: true}, 200 * time.Millisecond},
		{"response ending first", writeOK, time.Second, outcome{status: 200}, 0},
		{"disconnect before headers", neverRespond, 50 * time.Millisecond, outcome{disconnected: true}, 0},
		{"response failure", truncatedBody, time.Second, outcome{failed: true}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runRequest(testClient(t, time.Second), get(serve(t, tt.handler), tt.disconnectAfter))
			if got := outcomeOf(res); got != tt.want {
				t.Fatalf("result = %+v; want %+v", res, tt.want)
			}
			if res.Latency < tt.minLatency {
				t.Errorf("Latency = %v; want at least %v", res.Latency, tt.minLatency)
			}
		})
	}
}

func TestSessionDoesNotChangeOrdinaryRequestTimeout(t *testing.T) {
	url := serve(t, sseEvents)
	client := testClient(t, 50*time.Millisecond)
	requests := []*Request{get(url, 200*time.Millisecond), get(url, 0)}
	results := make(chan *Result, len(requests))
	for _, req := range requests {
		go executeRequest(client, req, time.Now(), results)
	}
	for range requests {
		res := <-results
		// The session outlives the 50ms client timeout; the ordinary request does not.
		want := outcome{failed: true}
		if res.Request.DisconnectAfter > 0 {
			want = outcome{status: 200, disconnected: true}
		}
		if got := outcomeOf(res); got != want {
			t.Errorf("result = %+v; want %+v", res, want)
		}
	}
	if client.Timeout != 50*time.Millisecond {
		t.Fatal("session changed the shared client timeout")
	}
}

func TestNonpositiveTimeoutDisablesDeadline(t *testing.T) {
	url := serve(t, writeOK)
	for _, timeout := range []time.Duration{0, -time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			res := runRequest(testClient(t, timeout), get(url, 0))
			if got := outcomeOf(res); got != (outcome{status: 200}) {
				t.Fatalf("result = %+v; want no deadline", res)
			}
		})
	}
}

func TestSilentSessionClosesAtRequestedDuration(t *testing.T) {
	closed := make(chan struct{})
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(closed)
	})
	res := runRequest(testClient(t, 50*time.Millisecond), get(url, 200*time.Millisecond))
	if got := outcomeOf(res); got != (outcome{status: 200, disconnected: true}) {
		t.Fatalf("result = %+v; want legal silence until the session deadline", res)
	}
	if res.Latency < 200*time.Millisecond {
		t.Errorf("Latency = %v; want at least 200ms", res.Latency)
	}
	waitClosed(t, closed)
}

// waitClosed fails the test unless the server closes closed within 5s,
// showing that it saw the client go.
func waitClosed(t *testing.T, closed <-chan struct{}) {
	t.Helper()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("server session remained open after disconnect")
	}
}

func TestReplayHeadersAndRedirectBehavior(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Last-Event-ID"); got != "42" {
			t.Errorf("Last-Event-ID = %q; want 42", got)
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("Accept = %q; want text/event-stream", got)
		}
		w.Header().Set("Location", "/must-not-follow")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	res := runRequest(testClient(t, time.Second), &Request{
		Method: "GET", Url: srv.URL,
		Headers: map[string]string{"Accept": "text/event-stream", "Last-Event-ID": "42"},
	})
	if res.ErrorMsg != "" || res.StatusCode != http.StatusFound {
		t.Fatalf("result = %+v; want the redirect response without following it", res)
	}
}

func TestDryRunDoesNotWaitForSessionDuration(t *testing.T) {
	requests := make(chan *Request, 1)
	results := make(chan *Result, 1)
	requests <- &Request{DisconnectAfter: Duration(time.Hour)}
	close(requests)
	doHttpRequest(testClient(t, time.Second), requests, results, true)
	if res := <-results; res.Disconnected || res.ErrorMsg != "" {
		t.Fatalf("dry-run result = %+v; want no disconnect or error", res)
	}
}

func BenchmarkOrdinaryRequest(b *testing.B) {
	body := make([]byte, 16<<10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	b.Cleanup(srv.Close)
	client := testClient(b, 10*time.Second)
	req := &Request{Method: "GET", Url: srv.URL}
	results := make(chan *Result, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		executeRequest(client, req, time.Now(), results)
		if res := <-results; res.ErrorMsg != "" {
			b.Fatal(res.ErrorMsg)
		}
	}
}

func TestOrdinaryResultOmitsDisconnectField(t *testing.T) {
	data, err := json.Marshal(&Result{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "disconnected") {
		t.Fatalf("ordinary JSON result includes disconnected: %s", data)
	}
}
