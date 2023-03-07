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
	"testing"
	"time"
)

func TestUnrmarshalInvalidMethod(t *testing.T) {
	jsonRequest := `{"method": "WHAT"}`
	req, err := unmarshalRequest([]byte(jsonRequest))

	if req != nil {
		t.Errorf("req = %v; want nil", req)
	}

	if err.Error() != "Invalid method: WHAT" {
		t.Errorf(`err.Error() = %v; want "Invalid method: WHAT"`, err.Error())
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

func TestFasthttpRequest(t *testing.T) {
	jsonRequest := `
	{
		"method": "GET", "url": "http://example.com", "body":"body", "timestamp": "2021-11-08T18:59:59.9Z",
		"headers": {"Content-Type": "application/json", "Host": "example.net", "Cookies":"cookie=1234567890", "User-Agent":"Mozilla/5.0", "x-custom-header":"x-custom-header-value"}
	}
	`
	req, err := unmarshalRequest([]byte(jsonRequest))
	if err != nil {
		t.Errorf("err = %v; want nil", err)
	}

	fr := req.fasthttpRequest()

	if string(fr.Header.Method()) != "GET" {
		t.Errorf("Method = %v; want GET", req.Method)
	}

	if string(fr.Body()) != "body" {
		t.Errorf("Body = %v; want body", req.Method)
	}

	if string(fr.Header.ContentType()) != "application/json" {
		t.Errorf("ContentType = %s; want application/json", fr.Header.ContentType())
	}

	if string(fr.Header.UserAgent()) != "Mozilla/5.0" {
		t.Errorf("UserAgent = %s; want Mozilla/5.0", fr.Header.UserAgent())
	}

	if string(fr.Header.Peek("x-custom-header")) != "x-custom-header-value" {
		t.Errorf("UserAgent = %s; want Mozilla/5.0", fr.Header.UserAgent())
	}

	if string(fr.Header.Peek("Host")) != "example.net" {
		t.Errorf("Host = %s; want example.net", fr.Header.Peek("Host"))
	}

	if string(fr.Host()) != "example.net" {
		t.Errorf("Host = %s; want example.net", fr.Host())
	}

	if string(fr.Header.RequestURI()) != "http://example.com" {
		t.Errorf("RequestURI = %s; want http://example.com", fr.Header.RequestURI())
	}

	if string(fr.RequestURI()) != "/" {
		t.Errorf("RequestURI = %s; want /", fr.RequestURI())
	}
}
