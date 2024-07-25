package ripley

import (
	"context"
	"errors"
	"time"
)

func NewPhaseTicker(ctx context.Context, phasesStr string) (<-chan time.Time, error) {
	phases, err := parsePhases(phasesStr)

	if err != nil {
		return nil, err
	}

	if len(phases) == 0 {
		return nil, errors.New("no phases for NewPhaseTicker")
	}

	tickerChan := make(chan time.Time)
	go func() {
		defer close(tickerChan)
	Loop:
		for _, interval := range phases {
			ticker := time.NewTicker(time.Second / time.Duration(interval.rate))
			timer := time.NewTicker(interval.duration)

			for {
				select {
				case t := <-ticker.C:
					tickerChan <- t
				case <-timer.C:
					ticker.Stop()
					continue Loop
				case <-ctx.Done():
					ticker.Stop()
					return
				}
			}
		}
	}()

	return tickerChan, nil
}
