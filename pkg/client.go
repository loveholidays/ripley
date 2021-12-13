package ripley

import (
	"net/http"
	"time"
)

type result struct {
	StatusCode int           `json:"statusCode"`
	Latency    time.Duration `json:"latency"`
	Request    *request      `json:"request"`
	err        error
}

func startClientWorkers(numWorkers int, requests <-chan *request, results chan<- *result) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for i := 0; i <= numWorkers; i++ {
		go doHttpRequest(client, requests, results)
	}
}

func doHttpRequest(client *http.Client, requests <-chan *request, results chan<- *result) {
	for req := range requests {
		latencyStart := time.Now()
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

		latency := time.Now().Sub(latencyStart)
		results <- &result{StatusCode: resp.StatusCode, Latency: latency, Request: req}
	}
}
