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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var (
	validMethods = [9]string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}
)

type Request struct {
	Method    string            `json:"method"`
	Url       string            `json:"url"`
	Body      string            `json:"body"`
	Timestamp time.Time         `json:"timestamp"`
	Headers   map[string]string `json:"headers"`
}

func (r *Request) httpRequest() (*http.Request, error) {
	req, err := http.NewRequest(r.Method, r.Url, bytes.NewReader([]byte(r.Body)))

	if err != nil {
		return nil, err
	}

	for k, v := range r.Headers {
		req.Header.Add(k, v)
	}

	if host := req.Header.Get("Host"); host != "" {
		req.Host = host
	}

	return req, nil
}

func unmarshalRequest(jsonRequest []byte) (*Request, error) {
	req := &Request{}
	err := json.Unmarshal(jsonRequest, &req)

	if err != nil {
		return req, err
	}

	// Validate
	if !validMethod(req.Method) {
		return req, fmt.Errorf("Invalid method: %s", req.Method)
	}

	if req.Url == "" {
		return req, fmt.Errorf("Missing required key: url")
	}

	if req.Timestamp.IsZero() {
		return req, fmt.Errorf("Missing required key: timestamp")
	}

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
