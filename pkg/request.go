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
	"time"
	"unsafe"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
)

var (
	validMethods = [9]string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}
)

type request struct {
	Method    string            `json:"method"`
	Url       string            `json:"url"`
	Body      string            `json:"body"`
	Timestamp time.Time         `json:"timestamp"`
	Headers   map[string]string `json:"headers"`
}

func (r *request) fasthttpRequest() *fasthttp.Request {
	req := fasthttp.AcquireRequest()

	req.Header.SetMethod(r.Method)
	req.SetRequestURI(r.Url)
	req.SetBody([]byte(r.Body))

	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	if host := req.Header.Peek("Host"); len(host) > 0 {
		req.SetHost(string(host))
	}

	return req
}

func unmarshalRequest(jsonRequest []byte) (*request, error) {
	var p fastjson.Parser
	v, err := p.ParseBytes(jsonRequest)
	if err != nil {
		return nil, err
	}

	req := &request{
		Method:  string(v.GetStringBytes("method")),
		Url:     string(v.GetStringBytes("url")),
		Body:    string(v.GetStringBytes("body")),
		Headers: make(map[string]string),
	}

	// Validate
	if !validMethod(req.Method) {
		return nil, fmt.Errorf("invalid method: %s", req.Method)
	}

	// Parse headers
	headers := v.GetObject("headers")
	headers.Visit(func(k []byte, v *fastjson.Value) {
		req.Headers[string(k)] = string(v.GetStringBytes())
	})

	if req.Url == "" {
		return nil, fmt.Errorf("missing required key: url")
	}

	timestampVal := v.GetStringBytes("timestamp")
	if timestampVal == nil {
		return req, fmt.Errorf("missing required key: timestamp")
	}

	timestamp, err := time.Parse(time.RFC3339Nano, b2s(timestampVal))
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %v", timestamp)
	}
	req.Timestamp = timestamp

	return req, nil
}

func validMethod(requestMethod string) bool {
	for _, method := range validMethods {
		if requestMethod == method {
			return true
		}
	}
	return false
}

func doHttpRequest(opts Options, requests <-chan *request, results chan<- *Result) {
	for req := range requests {
		latencyStart := time.Now()
		if opts.DryRun {
			measureResult(opts, req, &fasthttp.Response{}, latencyStart, nil, results)
		} else {
			httpReq := req.fasthttpRequest()
			httpResp := fasthttp.AcquireResponse()

			client, err := getOrCreateHttpClient(opts, req)
			if err != nil {
				measureResult(opts, req, httpResp, latencyStart, err, results)
				return
			}

			err = client.DoTimeout(httpReq, httpResp, time.Duration(opts.Timeout)*time.Second)
			measureResult(opts, req, httpResp, latencyStart, err, results)

			fasthttp.ReleaseRequest(httpReq)
			fasthttp.ReleaseResponse(httpResp)
		}
	}
}

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
