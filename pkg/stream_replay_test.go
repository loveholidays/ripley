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
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReplayMixesSSEAndShortRequests(t *testing.T) {
	closed := make(chan struct{})
	srv := newMixedServer(t, closed)
	timestamp := time.Date(2021, 11, 8, 18, 59, 58, 0, time.UTC)
	results := replay(t, "1s@1",
		&Request{Method: "GET", Url: srv.URL + "/sse", Timestamp: timestamp, DisconnectAfter: Duration(200 * time.Millisecond)},
		&Request{Method: "GET", Url: srv.URL + "/short", Timestamp: timestamp},
	)
	if len(results) != 2 {
		t.Fatalf("got %d results; want 2", len(results))
	}
	if n := countDisconnects(t, results); n != 1 {
		t.Fatalf("disconnects = %d; want one stream and one normal completion", n)
	}
	waitClosed(t, closed)
}

// countDisconnects counts the disconnected results, failing the test if any
// result is not a successful 200.
func countDisconnects(t *testing.T, results []Result) int {
	t.Helper()
	n := 0
	for _, res := range results {
		if succeeded := res.ErrorMsg == "" && res.StatusCode == 200; !succeeded {
			t.Fatalf("unexpected result: %+v", res)
		}
		if res.Disconnected {
			n++
		}
	}
	return n
}

// newMixedServer answers /sse with a stream that stays open until the client
// leaves, then closes closed. Every other path gets a short response.
func newMixedServer(t *testing.T, closed chan<- struct{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sse" {
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: event\n\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(closed)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// replay runs Replay at pace with requests on stdin and no cap on requests in
// flight, and returns the decoded results.
func replay(t *testing.T, pace string, requests ...*Request) []Result {
	t.Helper()
	input := tempFile(t, "input")
	encoder := json.NewEncoder(input)
	for _, req := range requests {
		if err := encoder.Encode(req); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := input.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	output := tempFile(t, "output")
	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = input, output
	defer func() { os.Stdin, os.Stdout = oldIn, oldOut }()
	if code := Replay(pace, false, false, 10, false, 0, 10, 0, false, 0, false, ""); code != 0 {
		t.Fatalf("exit code = %d; want zero", code)
	}
	return readResults(t, output.Name())
}

func tempFile(t *testing.T, pattern string) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), pattern)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// readResults decodes the JSONL results in the named file.
func readResults(t *testing.T, name string) []Result {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var results []Result
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var res Result
		if err := json.Unmarshal([]byte(line), &res); err != nil {
			t.Fatal(err)
		}
		results = append(results, res)
	}
	return results
}

func TestReplaySessionsFollowLogTime(t *testing.T) {
	url := serve(t, sseEvents)
	tests := map[string]struct {
		pace     string
		min, max time.Duration // bounds on the session's wall time
	}{
		// 2s of log at 10x is 200ms of wall time.
		"10x": {"10s@10", 150 * time.Millisecond, time.Second},
		// The first 100ms of wall time replays 1s of log at 10x; the other
		// 1s of the session then runs at 1x, ending 1.1s in.
		"spans a phase change": {"100ms@10 10s@1", time.Second, 1600 * time.Millisecond},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := replaySession(t, url, tc.pace); got < tc.min || got > tc.max {
				t.Errorf("session lasted %v; want %v to %v", got, tc.min, tc.max)
			}
		})
	}
}

// replaySession replays one SSE session logged as lasting 2s, and returns how
// long it lasted in wall time. The session must end in an intentional
// disconnect.
func replaySession(t *testing.T, url, pace string) time.Duration {
	t.Helper()
	timestamp := time.Date(2021, 11, 8, 18, 59, 58, 0, time.UTC)
	results := replay(t, pace, &Request{
		Method: "GET", Url: url, Timestamp: timestamp, DisconnectAfter: Duration(2 * time.Second),
	})
	if len(results) != 1 {
		t.Fatalf("got %d results; want 1", len(results))
	}
	res := results[0]
	if got := outcomeOf(&res); got != (outcome{status: 200, disconnected: true}) {
		t.Fatalf("result = %+v; want an intentional disconnect", res)
	}
	return res.Latency
}

// The replay must keep its schedule however long responses take: 50 requests
// logged at once must all be open at the server together.
func TestReplayDoesNotWaitForResponses(t *testing.T) {
	srv := newBlockingServer(t)
	const n = 50
	timestamp := time.Date(2021, 11, 8, 18, 59, 58, 0, time.UTC)
	reqs := make([]*Request, n)
	for i := range reqs {
		reqs[i] = &Request{Method: "GET", Url: srv.URL, Timestamp: timestamp}
	}

	allOpen := make(chan bool, 1)
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for srv.open.Load() < n && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		allOpen <- srv.open.Load() == n
		srv.releaseAll()
	}()

	if results := replay(t, "10s@1", reqs...); len(results) != n {
		t.Fatalf("got %d results; want %d", len(results), n)
	}
	if !<-allOpen {
		t.Errorf("max open requests = %d; want all %d at once", srv.maxOpen.Load(), n)
	}
}
