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
	"time"
)

// timeline maps log time, the timestamps in the input, onto wall time. Once
// the first request anchors the two clocks, the pace phases fix the mapping:
// each phase lasts a set wall duration and replays log time at its rate. A
// timeline never changes after construction, so it is safe for concurrent use.
type timeline struct {
	segments []segment // one per phase, in order; never empty
	end      time.Time // wall time at which the last phase finishes
}

// segment places one phase on both clocks.
type segment struct {
	log, wall time.Time // where the phase starts on each clock
	rate      float64   // log time replayed per unit of wall time; positive
}

// newTimeline lays phases end to end from the anchor (logStart, wallStart).
// phases must be non-empty with positive rates, as parsePhases guarantees.
func newTimeline(phases []*phase, logStart, wallStart time.Time) timeline {
	segments := make([]segment, len(phases))
	log, wall := logStart, wallStart
	for i, p := range phases {
		segments[i] = segment{log: log, wall: wall, rate: p.rate}
		log = log.Add(durationOf(float64(p.duration) * p.rate))
		wall = wall.Add(p.duration)
	}
	return timeline{segments: segments, end: wall}
}

// wall returns the wall time at which log time t is replayed. Log times past
// the last phase continue at its rate, and log times before the anchor map
// to wall times before it.
func (tl timeline) wall(t time.Time) time.Time {
	s := tl.segmentAt(t)
	return s.wall.Add(durationOf(float64(t.Sub(s.log)) / s.rate))
}

// rateAt returns the replay rate in force at log time t.
func (tl timeline) rateAt(t time.Time) float64 {
	return tl.segmentAt(t).rate
}

// segmentAt returns the last segment starting at or before log time t, or
// the first segment if t precedes them all. A pace has only a few phases,
// so a linear scan is enough.
func (tl timeline) segmentAt(t time.Time) segment {
	s := tl.segments[0]
	for _, next := range tl.segments[1:] {
		if next.log.After(t) {
			break
		}
		s = next
	}
	return s
}

// durationOf converts nanoseconds to a Duration, saturating at its limits.
// A plain conversion of an out-of-range float is implementation-defined in Go,
// and extreme pace rates can produce one.
func durationOf(ns float64) time.Duration {
	switch {
	case ns >= math.MaxInt64: // float64(math.MaxInt64) rounds up to 2^63
		return math.MaxInt64
	case ns <= math.MinInt64:
		return math.MinInt64
	}
	return time.Duration(ns)
}
