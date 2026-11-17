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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockingServer holds every request open until release is closed, and
// tracks how many are open at once.
type blockingServer struct {
	*httptest.Server
	release       chan struct{}
	open, maxOpen atomic.Int64
}

func newBlockingServer(t *testing.T) *blockingServer {
	t.Helper()
	s := &blockingServer{release: make(chan struct{})}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := s.open.Add(1)
		defer s.open.Add(-1)
		for {
			old := s.maxOpen.Load()
			if n <= old || s.maxOpen.CompareAndSwap(old, n) {
				break
			}
		}
		<-s.release
	}))
	t.Cleanup(s.Close)
	return s
}

// runAll sends n requests to url through runRequests and returns once all
// results arrive.
func runAll(t *testing.T, url string, n int, limit inFlight, whileRunning func()) {
	t.Helper()
	client := testClient(t, 10*time.Second)
	client.Transport = &http.Transport{MaxIdleConnsPerHost: n}
	requests := make(chan *Request)
	results := make(chan *Result)
	go runRequests(client, requests, results, false, limit)

	var collected sync.WaitGroup
	collected.Add(1)
	go func() {
		defer collected.Done()
		for range n {
			if res := <-results; res.ErrorMsg != "" {
				t.Errorf("request failed: %s", res.ErrorMsg)
			}
		}
	}()

	sent := make(chan struct{})
	go func() {
		defer close(sent)
		for range n {
			requests <- &Request{Method: "GET", Url: url}
		}
		close(requests)
	}()
	whileRunning()
	<-sent
	collected.Wait()
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// Long-lived responses must not hold back the requests behind them, however
// many there are.
func TestRunRequestsDoesNotWaitForResponses(t *testing.T) {
	srv := newBlockingServer(t)
	const n = 200
	runAll(t, srv.URL, n, nil, func() {
		waitFor(t, "all requests to be open at once", func() bool { return srv.open.Load() == n })
		close(srv.release)
	})
}

func TestInFlightCapsConcurrentRequests(t *testing.T) {
	srv := newBlockingServer(t)
	const limit = 3
	runAll(t, srv.URL, 10, newInFlight(limit), func() {
		waitFor(t, "the cap to fill", func() bool { return srv.open.Load() == limit })
		time.Sleep(50 * time.Millisecond) // time for an extra request to slip through
		close(srv.release)
	})
	if got := srv.maxOpen.Load(); got != limit {
		t.Errorf("max open requests = %d; want %d", got, limit)
	}
}

// writeRecorder records each call to Write.
type writeRecorder struct{ writes []string }

func (w *writeRecorder) Write(p []byte) (int, error) {
	w.writes = append(w.writes, string(p))
	return len(p), nil
}

func TestLineWriterWritesOnlyWholeLines(t *testing.T) {
	var rec writeRecorder
	lw := newLineWriter(&rec)
	var want strings.Builder
	for _, size := range []int{10, 40 << 10, 30 << 10, 200 << 10, 5} { // some past the 64KB buffer
		line := strings.Repeat("x", size) + "\n"
		want.WriteString(line)
		if err := lw.WriteLine([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if err := lw.Flush(); err != nil {
		t.Fatal(err)
	}

	for i, w := range rec.writes {
		if !strings.HasSuffix(w, "\n") {
			t.Errorf("write %d of %d ends mid-line", i+1, len(rec.writes))
		}
	}
	if got := strings.Join(rec.writes, ""); got != want.String() {
		t.Errorf("output differs from the lines written")
	}
}

func TestReceiveOrFlushHoldsOutputOnlyWhileBusy(t *testing.T) {
	var out bytes.Buffer
	lw := newLineWriter(&out)
	results := make(chan *Result, 1)

	_ = lw.WriteLine([]byte("first\n"))
	results <- &Result{}
	if _, ok := receiveOrFlush(results, lw); !ok || out.Len() != 0 {
		t.Fatalf("flushed %q with a result waiting; want the line held", out.String())
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		results <- &Result{}
	}()
	if _, ok := receiveOrFlush(results, lw); !ok || out.String() != "first\n" {
		t.Fatalf("output = %q after waiting; want the line flushed while idle", out.String())
	}
}
