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
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestUnrmarshalInvalidMethod(t *testing.T) {
	jsonRequest := `{"method": "WHAT"}`
	_, err := unmarshalRequest([]byte(jsonRequest))

	if err.Error() != "invalid method: WHAT" {
		t.Errorf(`err.Error() = %v; want "invalid method: WHAT"`, err.Error())
	}
}

func TestUnrmarshalValid(t *testing.T) {
	jsonRequest := `{"method": "GET", "url": "http://example.com", "timestamp": "2021-11-08T18:59:59.9Z"}`
	req, err := unmarshalRequest([]byte(jsonRequest))

	if err != nil {
		t.Errorf("err = %v; want nil", err)
	}

	if req.Method != "GET" {
		t.Errorf("req.Method = %v; want GET", req.Method)
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

func TestUnmarshalClientBehaviour(t *testing.T) {
	jsonRequest := `{"method": "GET", "url": "http://example.com/sse", "timestamp": "2021-11-08T18:59:59.9Z", "disconnectAfter": "1m30s"}`
	req, err := unmarshalRequest([]byte(jsonRequest))
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}

	if time.Duration(req.DisconnectAfter) != 90*time.Second {
		t.Errorf("req.DisconnectAfter = %v; want 1m30s", time.Duration(req.DisconnectAfter))
	}
}

func TestUnmarshalInvalidClientBehaviour(t *testing.T) {
	tests := map[string]string{
		"numeric duration":  `"disconnectAfter": 30`,
		"bad duration":      `"disconnectAfter": "soon"`,
		"negative duration": `"disconnectAfter": "-1s"`,
	}

	for name, field := range tests {
		t.Run(name, func(t *testing.T) {
			jsonRequest := `{"method": "GET", "url": "http://example.com", "timestamp": "2021-11-08T18:59:59.9Z", ` + field + `}`
			if _, err := unmarshalRequest([]byte(jsonRequest)); err == nil {
				t.Error("err = nil; want an error")
			}
		})
	}
}

func TestDurationMarshalsAsString(t *testing.T) {
	b, err := json.Marshal(&Request{DisconnectAfter: Duration(30 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"disconnectAfter":"30s"`) {
		t.Errorf("json = %s; want duration as \"30s\"", b)
	}

	b, _ = json.Marshal(&Request{})
	if strings.Contains(string(b), "disconnectAfter") {
		t.Errorf("json = %s; want zero duration omitted", b)
	}
}

func TestNullDisconnectAfterMeansNotSet(t *testing.T) {
	jsonRequest := `{"method": "GET", "url": "http://example.com", "timestamp": "2021-11-08T18:59:59.9Z", "disconnectAfter": null}`
	req, err := unmarshalRequest([]byte(jsonRequest))
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}
	if req.DisconnectAfter != 0 {
		t.Errorf("req.DisconnectAfter = %v; want 0", time.Duration(req.DisconnectAfter))
	}
}
