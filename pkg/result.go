package ripley

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fasthttp"
)

type Result struct {
	StatusCode int           `json:"statusCode"`
	Latency    time.Duration `json:"latency"`
	Request    *request      `json:"request"`
	ErrorMsg   string        `json:"error"`
}

func measureResult(opts Options, req *request, resp *fasthttp.Response, latencyStart time.Time, err error, results chan<- *Result) {
	latency := time.Since(latencyStart)
	if err != nil {
		results <- &Result{StatusCode: -1, Latency: latency, Request: req, ErrorMsg: err.Error()}
	} else {
		results <- &Result{StatusCode: resp.StatusCode(), Latency: latency, Request: req, ErrorMsg: ""}
	}
}

func handleResult(opts Options, results <-chan *Result) {
	for result := range results {
		waitGroupResults.Done()

		requestDuration.Update(result.Latency.Seconds())
		metrics.GetOrCreateCounter(fmt.Sprintf(`response_code{status="%d"}`, result.StatusCode)).Inc()

		if !opts.Silent {
			jsonResult, err := json.Marshal(result)

			if err != nil {
				panic(err)
			}

			fmt.Println(string(jsonResult))
		}
	}
}
