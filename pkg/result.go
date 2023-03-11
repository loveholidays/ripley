package ripley

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/valyala/fasthttp"
)

type Result struct {
	StatusCode int           `json:"StatusCode"`
	Latency    time.Duration `json:"Latency"`
	Request    *request      `json:"Request"`
	ErrorMsg   string        `json:"Error"`
}

func (r *Result) toJson() string {
	j, err := json.Marshal(r)

	if err != nil {
		panic(err)
	}

	return b2s(j)
}

func measureResult(opts *Options, req *request, resp *fasthttp.Response, latencyStart time.Time, err error, results chan<- *Result) {
	latency := time.Since(latencyStart)
	var statusCode int
	var errorMsg string

	switch {
	case err != nil:
		statusCode = -1
		errorMsg = err.Error()
	default:
		statusCode = resp.StatusCode()
		errorMsg = ""
	}

	results <- &Result{
		StatusCode: statusCode,
		Latency:    latency,
		Request:    req,
		ErrorMsg:   errorMsg,
	}
}

func handleResult(opts *Options, results <-chan *Result) {
	for result := range results {
		metricHandleResult(result)
		storeLongestResults(result, opts)

		if !opts.Silent {
			fmt.Println(result.toJson())
		}

		waitGroupResults.Done()
	}
}
