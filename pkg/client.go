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
	"io"
	"net/http"
	"time"
)

type Result struct {
	StatusCode int           `json:"statusCode"`
	Latency    time.Duration `json:"latency"`
	Request    *Request      `json:"Request"`
	ErrorMsg   string        `json:"error"`
}

func startClientWorkers(numWorkers int, requests <-chan *Request, results chan<- *Result, dryRun bool, timeout, connections, maxConnections int) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			MaxIdleConnsPerHost: connections,
			MaxConnsPerHost:     maxConnections,
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
			httpReq, err := req.httpRequest()

			if err != nil {
				sendResult(req, &http.Response{}, latencyStart, err.Error(), results)
				return
			}

			resp, err := client.Do(httpReq)

			if err != nil {
				sendResult(req, &http.Response{}, latencyStart, err.Error(), results)
				return
			}

			_, err = io.ReadAll(resp.Body)
			if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
				err = closeErr
			}

			if err != nil {
				sendResult(req, &http.Response{}, latencyStart, err.Error(), results)
				return
			}

			sendResult(req, resp, latencyStart, "", results)
		}
	}
}

func sendResult(req *Request, resp *http.Response, latencyStart time.Time, err string, results chan<- *Result) {
	latency := time.Since(latencyStart)
	results <- &Result{StatusCode: resp.StatusCode, Latency: latency, Request: req, ErrorMsg: err}
}
