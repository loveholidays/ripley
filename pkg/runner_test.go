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
	"fmt"
	"net/http"
	"net/http/httptest"
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

// runAll sends n requests to s through runRequests, capped by limit if it is
// positive. Once at
// least want requests are open at once, it waits a little longer for any extra
// request to slip through, then releases them all and waits for every result.
func (s *blockingServer) runAll(t *testing.T, n, limit int, want int64) {
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
	srv.runAll(t, n, 0, n)
}

func TestInFlightCapsConcurrentRequests(t *testing.T) {
	srv := newBlockingServer(t)
	const limit = 3
	srv.runAll(t, 10, limit, limit)
	if got := srv.maxOpen.Load(); got != limit {
		t.Errorf("max open requests = %d; want %d", got, limit)
	}
}
