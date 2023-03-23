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
	"strconv"
	"strings"
	"sync"
	"time"
)

type pacer struct {
	phases          []*phase
	lastRequestTime time.Time
	done            bool
	mutex           sync.RWMutex
}

type phase struct {
	duration time.Duration
	rate     float64
}

func NewPacer(phasesStr string) (*pacer, error) {
	phases, err := parsePhases(phasesStr)

	if err != nil {
		return nil, err
	}

	return &pacer{phases: phases}, nil
}

func (p *pacer) start() {
	// Run a timer for the first phase's duration
	updatePacerMetrics(p.phases[0])

	time.AfterFunc(p.phases[0].duration, p.onPhaseElapsed)
}

func (p *pacer) onPhaseElapsed() {
	// Pop phase
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.phases = p.phases[1:]

	if len(p.phases) == 0 {
		p.done = true
		updatePacerMetrics(&phase{duration: 0, rate: 0})
	} else {
		updatePacerMetrics(p.phases[0])

		// Create a timer with next phase
		time.AfterFunc(p.phases[0].duration, p.onPhaseElapsed)
	}
}

func (p *pacer) waitDuration(t time.Time) time.Duration {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	// Need to check as time.AfterFunc updates phases lengh
	if len(p.phases) == 0 {
		return 0
	}

	// If there are no more phases left, continue with the last phase's rate
	if p.lastRequestTime.IsZero() {
		p.lastRequestTime = t
	}

	duration := t.Sub(p.lastRequestTime)
	p.lastRequestTime = t
	return time.Duration(float64(duration) / p.phases[0].rate)
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
