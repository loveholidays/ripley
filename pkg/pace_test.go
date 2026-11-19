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

func TestSimpleParsePhases(t *testing.T) {
	phases, err := parsePhases("5m@2.5")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(phases) != 1 {
		t.Errorf("len(phases) = %v; want 1", len(phases))
	}

	// 5 minutes in nanoseconds
	if phases[0].duration != 5*time.Minute {
		t.Errorf("phases[0].duration = %v; want 5 minutes", phases[0].duration)
	}

	if phases[0].rate != 2.5 {
		t.Errorf("phases[0].rate = %v; want 2.5", phases[0].rate)
	}
}

func TestParseManyPhases(t *testing.T) {
	actualPhases, err := parsePhases("5m@2.5 20m@5 1h30m@10")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedPhases := []*phase{
		{5 * time.Minute, 2.5},
		{20 * time.Minute, 5.0},
		{time.Hour + 30*time.Minute, 10.0}}

	if len(actualPhases) != len(expectedPhases) {
		t.Errorf("len(actualPhases) = %v; want 3", len(expectedPhases))
	}

	for i, expectedPhase := range expectedPhases {
		if actualPhases[i].duration != expectedPhase.duration {
			t.Errorf("actualPhases[i].duration = %v; want %v",
				actualPhases[i].duration, expectedPhase.duration)
		}

		if actualPhases[i].rate != expectedPhase.rate {
			t.Errorf("actualPhases[i].rate = %v; want %v",
				actualPhases[i].rate, expectedPhase.rate)
		}
	}
}

func TestParsePhasesRejectsInvalidPhases(t *testing.T) {
	for _, pace := range []string{"1m@0", "1m@-1", "1m@NaN", "1m@Inf", "-1m@1", "10s", "10s@1  5s@2", ""} {
		if _, err := parsePhases(pace); err == nil {
			t.Errorf("parsePhases(%q) succeeded; want an error", pace)
		}
	}
}

func TestScheduleFollowsRate(t *testing.T) {
	tests := map[string]struct {
		pace string
		gap  time.Duration // log time between two requests
		want time.Duration // wall time between their sends
	}{
		"real time": {"30s@1", 2 * time.Second, 2 * time.Second},
		"10x":       {"30s@10", time.Second, time.Second / 10},
		"half":      {"30s@0.5", time.Second, 2 * time.Second},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// The wait is measured from a later now, so it is a little short.
			if wait := secondWait(t, tc.pace, tc.gap); !justUnder(wait, tc.want, 50*time.Millisecond) {
				t.Errorf("second wait = %v; want just under %v", wait, tc.want)
			}
		})
	}
}

// secondWait schedules two requests gap apart in log time at pace, and returns
// how long the second must wait. The first, which anchors the timeline, must
// be sent at once.
func secondWait(t *testing.T, pace string, gap time.Duration) time.Duration {
	t.Helper()
	p, err := newPacer(pace)
	if err != nil {
		t.Fatal(err)
	}
	logStart := time.Date(2021, 11, 8, 18, 59, 58, 0, time.UTC)
	if wait, ok := p.schedule(at(logStart)); !ok || wait != 0 {
		t.Fatalf("first wait = %v, %v; want 0", wait, ok)
	}
	wait, ok := p.schedule(at(logStart.Add(gap)))
	if !ok {
		t.Fatal("second request not scheduled")
	}
	return wait
}

// justUnder reports whether got is at most want and less than slack below it.
func justUnder(got, want, slack time.Duration) bool {
	return got <= want && got > want-slack
}

// justOver reports whether got is at least want and less than slack above it.
func justOver(got, want, slack time.Duration) bool {
	return got >= want && got < want+slack
}

func TestScheduleStopsAfterLastPhase(t *testing.T) {
	p, err := newPacer("10s@1")
	if err != nil {
		t.Fatal(err)
	}
	logStart := time.Now()
	if _, ok := p.schedule(at(logStart)); !ok {
		t.Fatal("first request not scheduled")
	}
	if _, ok := p.schedule(at(logStart.Add(11 * time.Second))); ok {
		t.Error("request after the last phase was scheduled")
	}
}

func TestScheduleStopsOncePaceIsOver(t *testing.T) {
	p, err := newPacer("1ms@1")
	if err != nil {
		t.Fatal(err)
	}
	logStart := time.Now()
	if _, ok := p.schedule(at(logStart)); !ok {
		t.Fatal("first request not scheduled")
	}
	time.Sleep(5 * time.Millisecond)
	// Inside the pace in log time, but the replay has run out of wall time.
	if _, ok := p.schedule(at(logStart)); ok {
		t.Error("request scheduled after the pace ended")
	}
}

// Report intervals under a second used to divide by zero.
func TestReportStatsWithSubSecondInterval(t *testing.T) {
	p, err := newPacer("1m@1")
	if err != nil {
		t.Fatal(err)
	}
	p.ReportInterval = 100 * time.Millisecond
	logStart := time.Now()
	for i := range 3 {
		if _, ok := p.schedule(at(logStart.Add(time.Duration(i) * 150 * time.Millisecond))); !ok {
			t.Fatal("request not scheduled")
		}
	}
}

// at returns a request logged at t.
func at(t time.Time) *Request {
	return &Request{Timestamp: t}
}

// A session ends where it ended in the log: at @10, 30s of log is 3s.
func TestScheduleSetsSessionDeadlineInLogTime(t *testing.T) {
	p, err := newPacer("1m@10")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	req := &Request{Timestamp: start, DisconnectAfter: Duration(30 * time.Second)}
	if _, ok := p.schedule(req); !ok {
		t.Fatal("request not scheduled")
	}
	// The timeline is anchored a moment after start, so the session ends a
	// little after it.
	if got := req.disconnectAt.Sub(start); !justOver(got, 3*time.Second, 50*time.Millisecond) {
		t.Errorf("session ends %v after the request; want just over 3s", got)
	}

	ordinary := at(start)
	if _, ok := p.schedule(ordinary); !ok || !ordinary.disconnectAt.IsZero() {
		t.Errorf("ordinary request disconnectAt = %v; want none", ordinary.disconnectAt)
	}
}

func BenchmarkPacerSchedule(b *testing.B) {
	p, err := newPacer("1h@1")
	if err != nil {
		b.Fatal(err)
	}
	t := time.Now()
	req := &Request{}
	b.ReportAllocs()
	for i := range b.N {
		req.Timestamp = t.Add(time.Duration(i))
		_, _ = p.schedule(req)
	}
}
