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
	"os"
	"strings"
	"testing"
	"time"
)

func TestReplayMixesSSEAndShortRequests(t *testing.T) {
	closed := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sse" {
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: event\n\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(closed)
	}))
	defer srv.Close()
	input, err := os.CreateTemp(t.TempDir(), "input")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = input.Close() }()
	output, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = output.Close() }()
	timestamp := time.Date(2021, 11, 8, 18, 59, 58, 0, time.UTC)
	encoder := json.NewEncoder(input)
	for _, req := range []*Request{
		{Method: "GET", Url: srv.URL + "/sse", Timestamp: timestamp, DisconnectAfter: Duration(200 * time.Millisecond)},
		{Method: "GET", Url: srv.URL + "/short", Timestamp: timestamp},
	} {
		if err := encoder.Encode(req); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := input.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = input, output
	defer func() { os.Stdin, os.Stdout = oldIn, oldOut }()
	if code := Replay("1s@1", false, false, 1, false, 2, 10, 0, false, 0, false, ""); code != 0 {
		t.Fatalf("exit code = %d; want zero", code)
	}
	data, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d results; want 2", len(lines))
	}
	var disconnects int
	for _, line := range lines {
		var res Result
		if err := json.Unmarshal([]byte(line), &res); err != nil {
			t.Fatal(err)
		}
		if res.ErrorMsg != "" || res.StatusCode != 200 {
			t.Fatalf("unexpected result: %+v", res)
		}
		if res.Disconnected {
			disconnects++
		}
	}
	if disconnects != 1 {
		t.Fatalf("disconnects = %d; want one stream and one normal completion", disconnects)
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("server session did not close")
	}
}
