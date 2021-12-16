package ripley

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var (
	validVerbs = [9]string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}
)

type request struct {
	Verb      string            `json:"verb"`
	Url       string            `json:"url"`
	Body      string            `json:"body"`
	Timestamp time.Time         `json:"timestamp"`
	Headers   map[string]string `json:"headers"`
}

func (r *request) httpRequest() (*http.Request, error) {
	req, err := http.NewRequest(r.Verb, r.Url, bytes.NewReader([]byte(r.Body)))

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

func unmarshalRequest(jsonRequest []byte) (*request, error) {
	req := &request{}
	err := json.Unmarshal(jsonRequest, &req)

	if err != nil {
		return nil, err
	}

	// Validate

	if !validVerb(req.Verb) {
		return nil, errors.New(fmt.Sprintf("Invalid verb: %s", req.Verb))
	}

	if req.Url == "" {
		return nil, errors.New("Missing required key: url")
	}

	if req.Timestamp.IsZero() {
		return nil, errors.New("Missing required key: timestamp")
	}

	return req, nil
}

func validVerb(requestVerb string) bool {
	for _, verb := range validVerbs {
		if requestVerb == verb {
			return true
		}
	}
	return false
}
