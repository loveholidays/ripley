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
	"net"
	"time"

	"github.com/valyala/fasthttp"
)

type Result struct {
	StatusCode int           `json:"statusCode"`
	Latency    time.Duration `json:"latency"`
	Request    *request      `json:"request"`
	ErrorMsg   string        `json:"error"`
}

func startClientWorkers(numWorkers int, requests <-chan *request, results chan *Result, dryRun bool, timeout int, silent bool) {
	client := &fasthttp.Client{
		Name:            "ripley",
		MaxConnsPerHost: numWorkers,
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialTimeout(addr, time.Duration(timeout)*time.Second)
		},
	}

	for i := 0; i < numWorkers; i++ {
		go doHttpRequest(client, requests, results, dryRun)
		go handleResult(results, silent)
	}
}

func doHttpRequest(client *fasthttp.Client, requests <-chan *request, results chan<- *Result, dryRun bool) {
	for req := range requests {
		latencyStart := time.Now()

		if dryRun {
			sendResult(req, &fasthttp.Response{}, latencyStart, "", results)
		} else {
			go func() {
				httpReq := req.fasthttpRequest()
				//Add the "Connection: keep-alive" header forcefully to servers that do not fully comply with HTTP1.1
				httpReq.Header.Set("Connection", "keep-alive")
				httpRes := fasthttp.AcquireResponse()
				defer func() {
					fasthttp.ReleaseRequest(httpReq)
					fasthttp.ReleaseResponse(httpRes)
				}()

				if err := client.Do(httpReq, httpRes); err != nil {
					sendResult(req, httpRes, latencyStart, err.Error(), results)
				} else {
					sendResult(req, httpRes, latencyStart, "", results)
				}
			}()
		}
	}
}

func sendResult(req *request, resp *fasthttp.Response, latencyStart time.Time, err string, results chan<- *Result) {
	latency := time.Since(latencyStart)
	results <- &Result{StatusCode: resp.StatusCode(), Latency: latency, Request: req, ErrorMsg: err}
}

func handleResult(results <-chan *Result, silent bool) {
	for result := range results {
		waitGroupResults.Done()

		if !silent {
			jsonResult, err := json.Marshal(result)

			if err != nil {
				panic(err)
			}

			fmt.Println(string(jsonResult))
		}
	}
}
