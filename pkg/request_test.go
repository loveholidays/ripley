package ripley

import (
	"testing"
	"time"
)

func TestUnrmarshalInvalidVerb(t *testing.T) {
	jsonRequest := `{"verb": "WHAT"}`
	req, err := unmarshalRequest([]byte(jsonRequest))

	if req != nil {
		t.Errorf("req = %v; want nil", req)
	}

	if err.Error() != "Invalid verb: WHAT" {
		t.Errorf(`err.Error() = %v; want "Invalid verb: WHAT"`, err.Error())
	}
}

func TestUnrmarshalValid(t *testing.T) {
	jsonRequest := `{"verb": "GET", "url": "http://example.com", "timestamp": "2021-11-08T18:59:59.9Z"}`
	req, err := unmarshalRequest([]byte(jsonRequest))

	if err != nil {
		t.Errorf("err = %v; want nil", err)
	}

	if req.Verb != "GET" {
		t.Errorf("req.Verb = %v; want GET", req.Verb)
	}

	if req.Url != "http://example.com" {
		t.Errorf("req.Url = %v; want http://example.com", req.Url)
	}

	expectedTime, err := time.Parse(time.RFC3339, "2021-11-08T18:59:59.9Z")

	if err != nil {
		panic(err)
	}

	if req.Timestamp != expectedTime {
		t.Errorf("req.Timestamp = %v; want %v", req.Timestamp, expectedTime)
	}
}
