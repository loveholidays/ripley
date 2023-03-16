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
	"net/url"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
)

var (
	validMethods = [9]string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}
)

type Request struct {
	Address   string            `json:"Address"`
	IsTLS     bool              `json:"IsTLS"`
	Method    string            `json:"Method"`
	Url       string            `json:"Url"`
	Body      string            `json:"Body"`
	Timestamp time.Time         `json:"Timestamp"`
	Headers   map[string]string `json:"Headers"`
}

func (r *Request) fasthttpRequest() *fasthttp.Request {
	req := fasthttp.AcquireRequest()

	req.Header.SetMethod(r.Method)
	req.SetRequestURI(r.Url)
	req.SetBody([]byte(r.Body))

	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	if host := req.Header.Peek("Host"); len(host) > 0 {
		req.SetHost(b2s(host))
	}

	return req
}

func unmarshalRequest(jsonRequest *[]byte) (*Request, error) {
	var p fastjson.Parser
	v, err := p.ParseBytes(*jsonRequest)
	if err != nil {
		return &Request{}, err
	}

	req := &Request{
		Method:  b2s(v.GetStringBytes("method")),
		Url:     b2s(v.GetStringBytes("url")),
		Body:    b2s(v.GetStringBytes("body")),
		Headers: make(map[string]string),
	}

	if req.Url == "" {
		return req, fmt.Errorf("missing required key: url")
	}

	// Parse URL
	up, err := url.Parse(req.Url)
	if err != nil {
		return req, err
	}
	req.Address = up.Host
	if up.Port() == "" {
		req.Address += ":80"
	}
	req.IsTLS = up.Scheme == "https"

	// Validate
	if !validMethod(req.Method) {
		return req, fmt.Errorf("invalid method: %s", req.Method)
	}

	// Parse headers
	headers := v.GetObject("headers")
	headers.Visit(func(k []byte, v *fastjson.Value) {
		req.Headers[b2s(k)] = b2s(v.GetStringBytes())
	})

	timestampVal := v.GetStringBytes("timestamp")
	if timestampVal == nil {
		return req, fmt.Errorf("missing required key: timestamp")
	}

	timestamp, err := time.Parse(time.RFC3339Nano, b2s(timestampVal))
	if err != nil {
		return req, fmt.Errorf("invalid timestamp: %v", timestamp)
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

func doHttpRequest(opts *Options, requests <-chan *Request, results chan<- *Result) {
	for req := range requests {
		latencyStart := time.Now()
		if opts.DryRun {
			measureResult(opts, req, &fasthttp.Response{}, latencyStart, nil, results)
		} else {
			httpReq := req.fasthttpRequest()
			httpResp := fasthttp.AcquireResponse()
			httpResp.SkipBody = true

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
