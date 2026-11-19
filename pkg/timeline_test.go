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
	"math"
	"testing"
	"time"
)

func TestTimelineWall(t *testing.T) {
	logStart := time.Date(2021, 11, 8, 18, 0, 0, 0, time.UTC)
	wallStart := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	// 10s of wall time at 2x replays 20s of log; then 1x for 10s.
	tl := newTimeline([]*phase{{10 * time.Second, 2}, {10 * time.Second, 1}}, logStart, wallStart)

	tests := map[string]struct {
		log  time.Duration // after logStart
		wall time.Duration // after wallStart
	}{
		"anchor":                    {0, 0},
		"first phase":               {4 * time.Second, 2 * time.Second},
		"phase boundary":            {20 * time.Second, 10 * time.Second},
		"second phase":              {25 * time.Second, 15 * time.Second},
		"end":                       {30 * time.Second, 20 * time.Second},
		"past the end at last rate": {40 * time.Second, 30 * time.Second},
		"before the anchor":         {-4 * time.Second, -2 * time.Second},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tl.wall(logStart.Add(tc.log)).Sub(wallStart)
			if got != tc.wall {
				t.Errorf("wall(start+%v) = start+%v; want start+%v", tc.log, got, tc.wall)
			}
		})
	}

	if got := tl.end.Sub(wallStart); got != 20*time.Second {
		t.Errorf("end = start+%v; want start+20s", got)
	}
}

// A session that spans a phase change must follow both rates, not just the
// rate in force when it began.
func TestTimelineSessionSpansPhases(t *testing.T) {
	logStart := time.Date(2021, 11, 8, 18, 0, 0, 0, time.UTC)
	wallStart := time.Now()
	tl := newTimeline([]*phase{{time.Second, 10}, {time.Minute, 1}}, logStart, wallStart)

	// 30s in the log: 10s pass in the first wall second, the other 20s at 1x.
	end := tl.wall(logStart.Add(30 * time.Second))
	if got := end.Sub(wallStart); got != 21*time.Second {
		t.Errorf("session lasts %v of wall time; want 21s", got)
	}
}

func TestTimelineZeroLengthPhase(t *testing.T) {
	logStart := time.Date(2021, 11, 8, 18, 0, 0, 0, time.UTC)
	wallStart := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	tl := newTimeline([]*phase{{0, 5}, {10 * time.Second, 1}}, logStart, wallStart)

	if got := tl.wall(logStart.Add(time.Second)).Sub(wallStart); got != time.Second {
		t.Errorf("wall = start+%v; want the 1x phase to apply: start+1s", got)
	}
}

func TestTimelineWallDoesNotAllocate(t *testing.T) {
	logStart := time.Now()
	tl := newTimeline([]*phase{{time.Minute, 2}, {time.Minute, 4}}, logStart, logStart)
	allocs := testing.AllocsPerRun(100, func() {
		_ = tl.wall(logStart.Add(90 * time.Second))
	})
	if allocs != 0 {
		t.Errorf("wall allocates %v times per call; want 0", allocs)
	}
}

// Converting an out-of-range float to an integer is implementation-defined in
// Go: amd64 gives the most negative value. Extreme rates must still map log
// time forwards, past the end of the pace, on every architecture.
func TestTimelineExtremeRatesSaturate(t *testing.T) {
	logStart := time.Date(2021, 11, 8, 18, 0, 0, 0, time.UTC)
	wallStart := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	slow := newTimeline([]*phase{{time.Minute, 1e-300}}, logStart, wallStart)
	if got := slow.wall(logStart.Add(time.Second)); !got.After(slow.end) {
		t.Errorf("at 1e-300, 1s of log maps to %v; want after the end, %v", got, slow.end)
	}

	fast := newTimeline([]*phase{{time.Minute, 1e300}, {time.Minute, 1}}, logStart, wallStart)
	if got := fast.segments[1].log; got.Before(logStart) {
		t.Errorf("at 1e300, the next phase starts at log time %v; want after %v", got, logStart)
	}
}

func TestDurationOfSaturates(t *testing.T) {
	tests := []struct {
		in   float64
		want time.Duration
	}{
		{1.5e9, 1500 * time.Millisecond},
		{1e304, math.MaxInt64},
		{math.Inf(1), math.MaxInt64},
		{-1e304, math.MinInt64},
		{math.Inf(-1), math.MinInt64},
	}
	for _, tc := range tests {
		if got := durationOf(tc.in); got != tc.want {
			t.Errorf("durationOf(%g) = %v; want %v", tc.in, got, tc.want)
		}
	}
}
