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
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/loveholidays/ripley/pkg/internal/slowio"
)

type Result struct {
	StatusCode int           `json:"statusCode"`
	Latency    time.Duration `json:"latency"`
	Request    *Request      `json:"Request"`
	ErrorMsg   string        `json:"error"`

	// Disconnected reports that the requested session duration elapsed before
	// the request completed. An intentional disconnect has an empty ErrorMsg.
	Disconnected bool `json:"disconnected,omitempty"`
}

// startRequestRunner runs each request from requests on its own goroutine,
// as net/http's server runs each connection, so a slow or long-lived response
// never delays the requests behind it. A positive maxInFlight caps the
// requests running at once; at the cap, the next request waits for one to
// finish, which holds back the replay.
func startRequestRunner(maxInFlight int, requests <-chan *Request, results chan<- *Result, dryRun bool, timeout, connections, maxConnections int, disableKeepAlives bool) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			MaxIdleConnsPerHost: connections,
			MaxConnsPerHost:     maxConnections,
			DisableKeepAlives:   disableKeepAlives,
		},
	}

	go runRequests(client, requests, results, dryRun, maxInFlight)
}

// runRequests starts a goroutine per request until requests is closed,
// with at most maxInFlight running at once if it is positive.
func runRequests(client *http.Client, requests <-chan *Request, results chan<- *Result, dryRun bool, maxInFlight int) {
	var running chan struct{} // a semaphore; nil means no cap
	if maxInFlight > 0 {
		running = make(chan struct{}, maxInFlight)
	}
	for req := range requests {
		if running != nil {
			running <- struct{}{}
		}
		go func() {
			results <- doRequest(client, req, dryRun)
			if running != nil {
				<-running
			}
		}()
	}
}

func doRequest(client *http.Client, req *Request, dryRun bool) *Result {
	start := time.Now()
	if dryRun {
		return &Result{Request: req, Latency: time.Since(start)}
	}
	return executeRequest(client, req, start)
}

var (
	// errSessionEnded is the cause recorded when a session reaches its
	// deadline, which is how an intentional disconnect is told apart from
	// other failures.
	errSessionEnded = errors.New("session ended")

	// errSessionMissed reports a session whose deadline passed while it
	// waited to start, so it was never sent.
	errSessionMissed = errors.New("session ended before the request was sent; raise or remove -workers")
)

func executeRequest(client *http.Client, req *Request, start time.Time) *Result {
	httpReq, err := req.httpRequest()
	if err != nil {
		return &Result{Request: req, Latency: time.Since(start), ErrorMsg: err.Error()}
	}

	if !req.disconnectAt.IsZero() {
		if !start.Before(req.disconnectAt) {
			return &Result{Request: req, Latency: time.Since(start), ErrorMsg: errSessionMissed.Error()}
		}
		var cancel context.CancelFunc
		client, httpReq, cancel = withSession(client, httpReq, req.disconnectAt)
		defer cancel()
	}

	status, err := discardResponse(client, httpReq, req.Network.bytesPerSecond())
	res := &Result{Request: req, Latency: time.Since(start), StatusCode: status}
	classify(httpReq.Context(), res, err)
	return res
}

// withSession bounds req by deadline instead of client.Timeout. It returns a
// copy of client so the shared client, used concurrently by other requests, is
// unchanged; the copy shares the transport and its connection pool.
func withSession(client *http.Client, req *http.Request, deadline time.Time) (*http.Client, *http.Request, context.CancelFunc) {
	ctx, cancel := context.WithDeadlineCause(req.Context(), deadline, errSessionEnded)
	session := *client
	session.Timeout = 0
	return &session, req.WithContext(ctx), cancel
}

// discardResponse sends req and consumes the body without retaining it, so
// memory stays bounded for long-lived streams such as SSE. A positive rate
// limits how fast the body is read, in bytes per second. It returns the
// status code, which is nonzero whenever response headers arrived.
func discardResponse(client *http.Client, req *http.Request, rate int64) (int, error) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	var body io.Reader = resp.Body
	if rate > 0 {
		body = slowio.NewReader(req.Context(), resp.Body, rate)
	}
	_, err = io.Copy(io.Discard, body)
	if closeErr := resp.Body.Close(); err == nil {
		err = closeErr
	}
	return resp.StatusCode, err
}

// classify records the outcome of err on res. Reaching the session deadline is
// the requested behaviour, so it keeps the status; other errors zero it, as before.
func classify(ctx context.Context, res *Result, err error) {
	switch {
	case err == nil:
	case sessionEnded(ctx):
		res.Disconnected = true
	default:
		res.StatusCode = 0
		res.ErrorMsg = err.Error()
	}
}

// sessionEnded reports whether ctx ended because its session deadline, set by
// withSession, was reached.
func sessionEnded(ctx context.Context) bool {
	return errors.Is(context.Cause(ctx), errSessionEnded)
}
