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

type result struct {
	StatusCode int           `json:"statusCode"`
	Latency    time.Duration `json:"latency"`
	Request    *request      `json:"request"`
	err        error
}

func startClientWorkers(numWorkers int, requests <-chan *request, results chan<- *result, dryRun bool) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for i := 0; i <= numWorkers; i++ {
		go doHttpRequest(client, requests, results, dryRun)
	}
}

func doHttpRequest(client *http.Client, requests <-chan *request, results chan<- *result, dryRun bool) {
	for req := range requests {
		latencyStart := time.Now()

		if dryRun {
			sendResult(req, &http.Response{}, latencyStart, results)
		} else {
			go func() {
				httpReq, err := req.httpRequest()

				if err != nil {
					results <- &result{err: err}
					return
				}

				resp, err := client.Do(httpReq)

				if err != nil {
					results <- &result{err: err}
					return
				}

				_, err = io.ReadAll(resp.Body)
				defer resp.Body.Close()

				if err != nil {
					results <- &result{err: err}
					return
				}

				sendResult(req, resp, latencyStart, results)
			}()
		}
	}
}

func sendResult(req *request, resp *http.Response, latencyStart time.Time, results chan<- *result) {
	latency := time.Now().Sub(latencyStart)
	results <- &result{StatusCode: resp.StatusCode, Latency: latency, Request: req}
}
