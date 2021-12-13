package ripley

import (
	"strconv"
	"strings"
	"time"
)

type pacer struct {
	phases          []*phase
	lastRequestTime time.Time
	done            bool
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
	// Run a timer for the first phase's duration
	time.AfterFunc(p.phases[0].duration, p.onPhaseElapsed)
}

func (p *pacer) onPhaseElapsed() {
	// Pop phase
	p.phases = p.phases[1:]

	if len(p.phases) == 0 {
		p.done = true
	} else {
		// Create a timer with next phase
		time.AfterFunc(p.phases[0].duration, p.onPhaseElapsed)
	}
}

func (p *pacer) waitDuration(t time.Time) time.Duration {
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
