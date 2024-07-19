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
	"io"
	"net/http"
	"sync"
	"time"
)

type Result struct {
	StatusCode int           `json:"statusCode"`
	Latency    time.Duration `json:"latency"`
	Request    *request      `json:"request"`
	ErrorMsg   string        `json:"error"`
}

type WorkerPool struct {
	client     *http.Client
	requests   <-chan *request
	results    chan<- *Result
	dryRun     bool
	maxWorkers int
	workers    int
	waitGroup  sync.WaitGroup
}

func NewWorkerPool(numWorkers int, maxWorkers int, requests <-chan *request, results chan<- *Result, dryRun bool, timeout int) *WorkerPool {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	pool := WorkerPool{
		client:     client,
		requests:   requests,
		results:    results,
		dryRun:     dryRun,
		maxWorkers: maxWorkers,
		workers:    0,
	}

	for i := 0; i < numWorkers; i++ {
		pool.StartWorker()
	}

	go func() {
		for {
			fmt.Printf("number of workers: %d/%d\n", pool.workers, pool.maxWorkers)
			time.Sleep(time.Second)
		}
	}()

	return &pool
}

func (w *WorkerPool) StartWorker() {
	w.workers++
	w.waitGroup.Add(1)
	go w.doHttpRequest()
}

func (w *WorkerPool) Wait() {
	w.waitGroup.Wait()
}

func (w *WorkerPool) doHttpRequest() {
	defer w.waitGroup.Done()
	for req := range w.requests {
		latencyStart := time.Now()

		if w.dryRun {
			sendResult(req, &http.Response{}, latencyStart, "", w.results)
		} else {
			httpReq, err := req.httpRequest()

			if err != nil {
				sendResult(req, &http.Response{}, latencyStart, err.Error(), w.results)
				return
			}

			resp, err := w.client.Do(httpReq)

			if err != nil {
				sendResult(req, &http.Response{}, latencyStart, err.Error(), w.results)
				return
			}

			_, err = io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				sendResult(req, &http.Response{}, latencyStart, err.Error(), w.results)
				return
			}

			sendResult(req, resp, latencyStart, "", w.results)
		}
	}
}

func sendResult(req *request, resp *http.Response, latencyStart time.Time, err string, results chan<- *Result) {
	latency := time.Now().Sub(latencyStart)
	results <- &Result{StatusCode: resp.StatusCode, Latency: latency, Request: req, ErrorMsg: err}
}
