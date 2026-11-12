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

// Request describes an HTTP request read from JSONL. Headers are replayed as
// supplied. Long-lived responses, including SSE, do not require a special type.
type Request struct {
	Method    string            `json:"method"`
	Url       string            `json:"url"`
	Body      string            `json:"body"`
	Timestamp time.Time         `json:"timestamp"`
	Headers   map[string]string `json:"headers"`

	// DisconnectAfter, when positive, cancels the request this long after
	// execution starts, including connection setup and body consumption. It
	// replaces the HTTP client's timeout for this request. If it expires
	// first, Result.Disconnected is true and ErrorMsg is empty. Zero uses the
	// ordinary client timeout; negative values are invalid.
	DisconnectAfter Duration `json:"disconnectAfter,omitempty"`
}

// Duration is a time.Duration encoded in JSON as a Go duration string such
// as "30s" or "5m".
type Duration time.Duration

// UnmarshalJSON parses a Go duration string. null leaves d unchanged, as
// encoding/json does for built-in types, so it reads as "not set".
func (d *Duration) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}

	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf(`duration must be a string such as "30s": %w`, err)
	}

	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*d = Duration(v)
	return nil
}

// MarshalJSON encodes d as a Go duration string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
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
		return req, fmt.Errorf("invalid method: %s", req.Method)
	}

	if req.Url == "" {
		return req, fmt.Errorf("missing required key: url")
	}

	if req.Timestamp.IsZero() {
		return req, fmt.Errorf("missing required key: timestamp")
	}

	if req.DisconnectAfter < 0 {
		return req, fmt.Errorf("disconnectAfter must not be negative")
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
