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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUnmarshalNetwork(t *testing.T) {
	const prefix = `{"method":"GET","url":"http://example.com","timestamp":"2021-11-08T18:59:58.9Z"`
	tests := map[string]Kbps{
		``:                     0, // omitted: unlimited
		`,"network":null`:      0, // no-op, so the zero value stays
		`,"network":"3g"`:      400,
		`,"network":"slow-4g"`: 1440,
		`,"network":"fast-4g"`: 8100,
		`,"network":750`:       750,
		`,"network":0`:         0,
		`,"network":10000000`:  maxKbps,
	}
	for field, want := range tests {
		req, err := unmarshalRequest([]byte(prefix + field + "}"))
		if err != nil || req.Network != want {
			t.Errorf("%s: Network = %d, %v; want %d", field, req.Network, err, want)
		}
	}
}

func TestUnmarshalNetworkRejectsInvalid(t *testing.T) {
	const prefix = `{"method":"GET","url":"http://example.com","timestamp":"2021-11-08T18:59:58.9Z","network":`
	for _, value := range []string{`"5g"`, `"3G"`, `"750"`, `1.5`, `-1`, `10000001`, `true`, `[]`} {
		if req, err := unmarshalRequest([]byte(prefix + value + "}")); err == nil {
			t.Errorf("network %s: accepted as %d; want an error", value, req.Network)
		}
	}
}

func TestNetworkEncodesAsKbps(t *testing.T) {
	b, err := json.Marshal(&Request{Network: 400})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"network":400`) {
		t.Errorf("json = %s; want network as a number", b)
	}
	if b, _ := json.Marshal(&Request{}); strings.Contains(string(b), "network") {
		t.Errorf("json = %s; want unlimited network omitted", b)
	}
}

func TestBytesPerSecond(t *testing.T) {
	if got := Kbps(400).bytesPerSecond(); got != 50_000 {
		t.Errorf("400 kbps = %d B/s; want 50000", got)
	}
}

func newBodyServer(t *testing.T, size int) *httptest.Server {
	t.Helper()
	body := make([]byte, size)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNetworkLimitsDownload(t *testing.T) {
	srv := newBodyServer(t, 25_000) // half a second at 3g
	res := runRequest(testClient(t, 10*time.Second), &Request{Method: "GET", Url: srv.URL, Network: presetKbps["3g"]})
	if res.ErrorMsg != "" || res.StatusCode != http.StatusOK {
		t.Fatalf("result = %+v; want success", res)
	}
	if res.Latency < 400*time.Millisecond {
		t.Errorf("Latency = %v; want about 500ms at 3g", res.Latency)
	}
}

func TestDisconnectAfterInterruptsThrottledRead(t *testing.T) {
	srv := newBodyServer(t, 250_000) // five seconds at 3g
	res := runRequest(testClient(t, 10*time.Second), &Request{
		Method: "GET", Url: srv.URL, Network: presetKbps["3g"], DisconnectAfter: Duration(300 * time.Millisecond),
	})
	if !res.Disconnected || res.ErrorMsg != "" || res.StatusCode != http.StatusOK {
		t.Fatalf("result = %+v; want an intentional disconnect", res)
	}
	if res.Latency > 3*time.Second {
		t.Errorf("Latency = %v; want the disconnect to interrupt throttling", res.Latency)
	}
}

func TestUnmarshalNullNetworkIsNoOp(t *testing.T) {
	k := Kbps(400)
	if err := json.Unmarshal([]byte("null"), &k); err != nil || k != 400 {
		t.Errorf("after null: %d, %v; want 400 unchanged", k, err)
	}
}
