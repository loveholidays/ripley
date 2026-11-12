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

func startClientWorkers(numWorkers int, requests <-chan *Request, results chan<- *Result, dryRun bool, timeout, connections, maxConnections int, disableKeepAlives bool) {
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

	for i := 0; i < numWorkers; i++ {
		go doHttpRequest(client, requests, results, dryRun)
	}
}

func doHttpRequest(client *http.Client, requests <-chan *Request, results chan<- *Result, dryRun bool) {
	for req := range requests {
		latencyStart := time.Now()

		if dryRun {
			sendResult(req, &http.Response{}, latencyStart, "", results)
		} else {
			executeRequest(client, req, latencyStart, results)
		}
	}
}

func executeRequest(client *http.Client, req *Request, latencyStart time.Time, results chan<- *Result) {
	httpReq, err := req.httpRequest()
	if err != nil {
		sendResult(req, &http.Response{}, latencyStart, err.Error(), results)
		return
	}

	if req.DisconnectAfter > 0 {
		var cancel context.CancelFunc
		client, httpReq, cancel = withSession(client, httpReq, latencyStart.Add(time.Duration(req.DisconnectAfter)))
		defer cancel()
	}

	status, err := discardResponse(client, httpReq)
	res := &Result{Request: req, Latency: time.Since(latencyStart), StatusCode: status}
	classify(httpReq.Context(), res, err)
	results <- res
}

// withSession bounds req by deadline instead of client.Timeout. It returns a
// copy of client so the shared client, used concurrently by other workers, is
// unchanged; the copy shares the transport and its connection pool.
func withSession(client *http.Client, req *http.Request, deadline time.Time) (*http.Client, *http.Request, context.CancelFunc) {
	ctx, cancel := context.WithDeadline(req.Context(), deadline)
	session := *client
	session.Timeout = 0
	return &session, req.WithContext(ctx), cancel
}

// discardResponse sends req and consumes the body without retaining it, so
// memory stays bounded for long-lived streams such as SSE. It returns the
// status code, which is nonzero whenever response headers arrived.
func discardResponse(client *http.Client, req *http.Request) (int, error) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	_, err = io.Copy(io.Discard, resp.Body)
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

// sessionEnded reports whether ctx hit its deadline. Only withSession sets a
// deadline on the request context; client.Timeout fails requests separately.
func sessionEnded(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.DeadlineExceeded)
}

func sendResult(req *Request, resp *http.Response, latencyStart time.Time, err string, results chan<- *Result) {
	latency := time.Since(latencyStart)
	results <- &Result{StatusCode: resp.StatusCode, Latency: latency, Request: req, ErrorMsg: err}
}
