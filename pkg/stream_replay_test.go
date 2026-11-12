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
	results := replay(t, []*Request{
		{Method: "GET", Url: srv.URL + "/sse", Timestamp: timestamp, DisconnectAfter: Duration(200 * time.Millisecond)},
		{Method: "GET", Url: srv.URL + "/short", Timestamp: timestamp},
	})
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

// replay runs Replay with requests on stdin and returns the decoded results.
func replay(t *testing.T, requests []*Request) []Result {
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
	if code := Replay("1s@1", false, false, 1, false, 2, 10, 0, false, 0, false, ""); code != 0 {
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
