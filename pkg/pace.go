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
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// pacer decides when each request is replayed, and reports pacing stats.
// Only the replay loop uses it, so it needs no locking.
type pacer struct {
	ReportInterval  time.Duration
	phases          []*phase
	timeline        timeline // anchored by the first call to schedule
	anchored        bool
	lastRequestTime time.Time // log time of the last request scheduled
	requestCounter  int
	nextReport      time.Time
}

type phase struct {
	duration time.Duration
	rate     float64
}

func newPacer(phasesStr string) (*pacer, error) {
	phases, err := parsePhases(phasesStr)

	if err != nil {
		return nil, err
	}

	return &pacer{phases: phases}, nil
}

// schedule returns how long to wait before sending req: until its place on
// the timeline, or zero or less if that has passed. It returns false once the
// pace is over, or if req falls after it. The first call anchors the
// timeline, so the first request is sent at once.
//
// A session ends where it ended in the log, so schedule also sets
// req.disconnectAt from DisconnectAfter, mapped onto the wall clock.
func (p *pacer) schedule(req *Request) (time.Duration, bool) {
	t, now := req.Timestamp, time.Now()
	if !p.anchored {
		p.anchor(t, now)
	}

	send := p.timeline.wall(t)
	if now.After(p.timeline.end) || send.After(p.timeline.end) {
		return 0, false
	}
	if req.DisconnectAfter > 0 {
		req.disconnectAt = p.timeline.wall(t.Add(time.Duration(req.DisconnectAfter)))
	}

	p.reportStats(now, send, t)
	p.lastRequestTime = t
	return send.Sub(now), true
}

func (p *pacer) anchor(logStart, wallStart time.Time) {
	p.timeline = newTimeline(p.phases, logStart, wallStart)
	p.anchored = true
	p.lastRequestTime = logStart
	if p.ReportInterval > 0 {
		p.nextReport = wallStart.Add(p.ReportInterval)
	}
}

// reportStats prints a line per elapsed report interval. t is the log time of
// the request being scheduled; its rate is looked up only when printing.
func (p *pacer) reportStats(now, expectedWallTime, t time.Time) {
	if p.ReportInterval <= 0 {
		return
	}

	for p.nextReport.Before(expectedWallTime) {
		fmt.Fprintf(os.Stderr, "report_time=%s skew_seconds=%f last_request_time=%s rate=%f expected_rps=%d\n",
			p.nextReport.Format(time.RFC3339),
			now.Sub(p.nextReport).Seconds(),
			p.lastRequestTime.Format(time.RFC3339),
			p.timeline.rateAt(t),
			int(float64(p.requestCounter)/p.ReportInterval.Seconds()))
		p.nextReport = p.nextReport.Add(p.ReportInterval)
		p.requestCounter = 0
	}
	p.requestCounter++
}

// Format is [duration]@[rate] [duration]@[rate]..."
// e.g. "5s@1 10m@2"
// It returns at least one phase, each with a non-negative duration and a
// positive, finite rate.
func parsePhases(phasesStr string) ([]*phase, error) {
	var phases []*phase

	for _, durationAtRate := range strings.Split(phasesStr, " ") {
		durationStr, rateStr, ok := strings.Cut(durationAtRate, "@")
		if !ok {
			return nil, fmt.Errorf("invalid phase %q: want [duration]@[rate], such as 10s@1", durationAtRate)
		}

		duration, err := time.ParseDuration(durationStr)

		if err != nil {
			return nil, err
		}

		rate, err := strconv.ParseFloat(rateStr, 64)

		if err != nil {
			return nil, err
		}

		if !validPhase(duration, rate) {
			return nil, fmt.Errorf("invalid phase %q: duration must not be negative and rate must be positive", durationAtRate)
		}

		phases = append(phases, &phase{duration, rate})
	}

	return phases, nil
}

// validPhase reports whether a phase can be placed on the timeline: log time
// must move forward, so the rate is positive and finite.
func validPhase(duration time.Duration, rate float64) bool {
	return duration >= 0 && rate > 0 && !math.IsInf(rate, 1)
}
