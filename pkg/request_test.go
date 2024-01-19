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
	_, err := unmarshalRequest([]byte(jsonRequest))

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
