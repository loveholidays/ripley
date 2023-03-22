package ripley

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/valyala/fasthttp"
)

type Response struct {
	StatusCode int                 `json:"StatusCode"`
	Headers    map[string][]string `json:"Headers"`
	RAddr      string              `json:"RAddr"`
	LAddr      string              `json:"LAddr"`
}

type Result struct {
	StatusCode int           `json:"StatusCode"`
	Latency    time.Duration `json:"Latency"`
	Request    *Request      `json:"Request"`
	Response   *Response     `json:"Response"`
	ErrorMsg   string        `json:"Error"`
}

func (r *Result) toJson() string {
	j, err := json.Marshal(r)

	if err != nil {
		panic(err)
	}

	return b2s(j)
}

func sendToResult(opts *Options, req *Request, resp *fasthttp.Response, latencyStart time.Time, err error, results chan<- *Result) {
	latency := time.Since(latencyStart)

	var errorMsg string
	var raddr, laddr string
	var statusCode int = resp.StatusCode()

	if err != nil {
		statusCode = -1
		errorMsg = err.Error()
	}

	respHeaders := make(map[string][]string)
	resp.Header.VisitAll(func(key, value []byte) {
		k := b2s(key)
		v := b2s(value)

		respHeaders[k] = append(respHeaders[k], v)
	})

	if adr := resp.RemoteAddr(); adr != nil {
		raddr = adr.String()
	}
	if adr := resp.LocalAddr(); adr != nil {
		laddr = adr.String()
	}

	results <- &Result{
		StatusCode: statusCode,
		Latency:    latency,
		Request:    req,
		Response: &Response{
			StatusCode: statusCode,
			Headers:    respHeaders,
			RAddr:      raddr,
			LAddr:      laddr,
		},
		ErrorMsg: errorMsg,
	}
}

// TODO: Consider rewriting the code to use a Result Broker with multi-channel and broadcast functionality in order to improve its scalability.
func handleResult(opts *Options, results <-chan *Result, slowestResults *SlowestResults) {
	for result := range results {
		metricHandleResult(result)
		slowestResults.storeResult(result)

		if !opts.Silent {
			fmt.Println(result.toJson())
		}

		if !opts.SilentHttpError && result.StatusCode < 0 || (result.StatusCode >= 500 && result.StatusCode <= 599) {
			fmt.Fprintln(os.Stderr, result.toJson())
		}

		waitGroupResults.Done()
	}
}
