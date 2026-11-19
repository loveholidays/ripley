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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockingServer holds every request open until releaseAll, and tracks how
// many are open at once.
type blockingServer struct {
	*httptest.Server
	release       chan struct{}
	releaseOnce   sync.Once
	open, maxOpen atomic.Int64
}

// releaseAll lets every held request, and any later one, finish.
func (s *blockingServer) releaseAll() {
	s.releaseOnce.Do(func() { close(s.release) })
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
	// Cleanups run last first: release held requests before Close waits for
	// them, so a failing test fails rather than hangs.
	t.Cleanup(s.releaseAll)
	return s
}

// runAll sends n requests to s through runRequests, capped by limit. Once at
// least want requests are open at once, it waits a little longer for any extra
// request to slip through, then releases them all and waits for every result.
func (s *blockingServer) runAll(t *testing.T, n int, limit inFlight, want int64) {
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

	go func() {
		for range n {
			requests <- &Request{Method: "GET", Url: s.URL}
		}
		close(requests)
	}()

	waitFor(t, fmt.Sprintf("%d requests to be open at once", want), func() bool { return s.open.Load() >= want })
	time.Sleep(50 * time.Millisecond)
	s.releaseAll()
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
	srv.runAll(t, n, nil, n)
}

func TestInFlightCapsConcurrentRequests(t *testing.T) {
	srv := newBlockingServer(t)
	const limit = 3
	srv.runAll(t, 10, newInFlight(limit), limit)
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
