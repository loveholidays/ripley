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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type pacer struct {
	ReportInterval        time.Duration
	mu                    sync.RWMutex // protects all fields below
	phases                []*phase
	lastRequestTime       time.Time // last Request that we already replayed in "log time"
	lastRequestWallTime   time.Time // last Request that we already replayed in "wall time"
	phaseStartRequestTime time.Time
	phaseStartWallTime    time.Time
	done                  bool
	requestCounter        int
	nextReport            time.Time
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

func (p *pacer) start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Run a timer for the first phase's duration
	if len(p.phases) > 0 {
		time.AfterFunc(p.phases[0].duration, p.onPhaseElapsed)
	}
	if p.ReportInterval > 0 {
		p.nextReport = time.Now().Add(p.ReportInterval)
	}
}

func (p *pacer) onPhaseElapsed() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Pop phase
	if len(p.phases) > 0 {
		p.phases = p.phases[1:]
	}
	p.phaseStartRequestTime = p.lastRequestTime
	p.phaseStartWallTime = p.lastRequestWallTime

	if len(p.phases) == 0 {
		p.done = true
	} else {
		// Create a timer with next phase
		time.AfterFunc(p.phases[0].duration, p.onPhaseElapsed)
	}
}

func (p *pacer) waitDuration(t time.Time) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	now := time.Now()

	if p.lastRequestTime.IsZero() {
		p.lastRequestTime = t
		p.lastRequestWallTime = now
		p.phaseStartRequestTime = p.lastRequestTime
		p.phaseStartWallTime = p.lastRequestWallTime
	}

	// Check if we have any phases left
	if len(p.phases) == 0 {
		return 0
	}

	originalDurationFromPhaseStart := t.Sub(p.phaseStartRequestTime)
	expectedDurationFromPhaseStart := time.Duration(float64(originalDurationFromPhaseStart) / p.phases[0].rate)
	expectedWallTime := p.phaseStartWallTime.Add(expectedDurationFromPhaseStart)

	p.reportStats(now, expectedWallTime)

	duration := expectedWallTime.Sub(now)
	p.lastRequestTime = t
	p.lastRequestWallTime = expectedWallTime
	return duration
}

func (p *pacer) isDone() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.done
}

func (p *pacer) reportStats(now, expectedWallTime time.Time) {
	if p.ReportInterval <= 0 {
		return
	}
	
	// Get current rate safely
	var currentRate float64 = 1.0
	if len(p.phases) > 0 {
		currentRate = p.phases[0].rate
	}
	
	for p.nextReport.Before(expectedWallTime) {
		fmt.Fprintf(os.Stderr, "report_time=%s skew_seconds=%f last_request_time=%s rate=%f expected_rps=%d\n",
			p.nextReport.Format(time.RFC3339),
			now.Sub(p.nextReport).Seconds(),
			p.lastRequestTime.Format(time.RFC3339),
			currentRate,
			p.requestCounter/int(p.ReportInterval.Seconds()))
		p.nextReport = p.nextReport.Add(p.ReportInterval)
		p.requestCounter = 0
	}
	p.requestCounter++
}

// Format is [duration]@[rate] [duration]@[rate]..."
// e.g. "5s@1 10m@2"
func parsePhases(phasesStr string) ([]*phase, error) {
	var phases []*phase

	for _, durationAtRate := range strings.Split(phasesStr, " ") {
		tokens := strings.Split(durationAtRate, "@")

		duration, err := time.ParseDuration(tokens[0])

		if err != nil {
			return nil, err
		}

		rate, err := strconv.ParseFloat(tokens[1], 64)

		if err != nil {
			return nil, err
		}

		phases = append(phases, &phase{duration, rate})
	}

	return phases, nil
}
