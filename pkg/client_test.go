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

func newSSEServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	t.Cleanup(srv.Close)
	return srv
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

func TestDisconnectAfterIsNotAnError(t *testing.T) {
	srv := newSSEServer(t)
	res := runRequest(testClient(t, time.Second), &Request{Method: "GET", Url: srv.URL, DisconnectAfter: Duration(200 * time.Millisecond)})
	if res.ErrorMsg != "" || !res.Disconnected || res.StatusCode != http.StatusOK {
		t.Fatalf("result = %+v; want an intentional disconnect with status 200", res)
	}
	if res.Latency < 200*time.Millisecond || res.Latency > 5*time.Second {
		// Generous upper bound: it only proves the session ended, on slow CI too.
		t.Errorf("Latency = %v; want about 200ms", res.Latency)
	}
}

func TestSessionDoesNotChangeOrdinaryRequestTimeout(t *testing.T) {
	srv := newSSEServer(t)
	client := testClient(t, 50*time.Millisecond)
	requests := []*Request{
		{Method: "GET", Url: srv.URL, DisconnectAfter: Duration(200 * time.Millisecond)},
		{Method: "GET", Url: srv.URL},
	}
	results := make(chan *Result, len(requests))
	for _, req := range requests {
		go executeRequest(client, req, time.Now(), results)
	}
	for range requests {
		res := <-results
		if res.Request.DisconnectAfter > 0 {
			if !res.Disconnected || res.ErrorMsg != "" || res.Latency < 200*time.Millisecond {
				t.Errorf("session result = %+v; want a disconnect after 200ms", res)
			}
		} else if res.ErrorMsg == "" || res.Disconnected || res.StatusCode != 0 {
			t.Errorf("ordinary result = %+v; want the existing timeout error", res)
		}
	}
	if client.Timeout != 50*time.Millisecond {
		t.Fatal("session changed the shared client timeout")
	}
}

func TestResponseEndingFirstIsNotDisconnected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("done"))
	}))
	defer srv.Close()
	res := runRequest(testClient(t, time.Second), &Request{Method: "GET", Url: srv.URL, DisconnectAfter: Duration(time.Second)})
	if res.Disconnected || res.ErrorMsg != "" || res.StatusCode != 200 {
		t.Fatalf("result = %+v; want a normal completion", res)
	}
}

func TestNonpositiveTimeoutDisablesDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	for _, timeout := range []time.Duration{0, -time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			res := runRequest(testClient(t, timeout), &Request{Method: "GET", Url: srv.URL})
			if res.ErrorMsg != "" || res.StatusCode != 200 {
				t.Fatalf("result = %+v; want no deadline", res)
			}
		})
	}
}

func TestDisconnectBeforeHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	res := runRequest(testClient(t, time.Second), &Request{Method: "GET", Url: srv.URL, DisconnectAfter: Duration(50 * time.Millisecond)})
	if !res.Disconnected || res.ErrorMsg != "" || res.StatusCode != 0 {
		t.Fatalf("result = %+v; want an intentional disconnect before headers", res)
	}
}

func TestSilentSessionClosesAtRequestedDuration(t *testing.T) {
	closed := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(closed)
	}))
	defer srv.Close()
	res := runRequest(testClient(t, 50*time.Millisecond), &Request{Method: "GET", Url: srv.URL, DisconnectAfter: Duration(200 * time.Millisecond)})
	if !res.Disconnected || res.ErrorMsg != "" || res.Latency < 200*time.Millisecond {
		t.Fatalf("result = %+v; want legal silence until the session deadline", res)
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("server session remained open after disconnect")
	}
}

func TestResponseFailureIsNotAnIntentionalDisconnect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("truncated"))
	}))
	defer srv.Close()
	res := runRequest(testClient(t, time.Second), &Request{Method: "GET", Url: srv.URL, DisconnectAfter: Duration(time.Second)})
	if res.ErrorMsg == "" || res.Disconnected || res.StatusCode != 0 {
		t.Fatalf("result = %+v; want a body-read failure", res)
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
