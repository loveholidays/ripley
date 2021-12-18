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
		&phase{5 * time.Minute, 2.5},
		&phase{20 * time.Minute, 5.0},
		&phase{time.Hour + 30*time.Minute, 10.0}}

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

func TestWaitDuration(t *testing.T) {
	pacer, err := newPacer("30s@1")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	now := time.Now()
	duration := pacer.waitDuration(now)

	if duration != 0 {
		t.Errorf("duration = %v; want 0", duration)
	}

	now = now.Add(2 * time.Second)
	duration = pacer.waitDuration(now)
	expected := 2 * time.Second

	if duration != expected {
		t.Errorf("duration = %v; want %v", duration, expected)
	}
}

func TestWaitDuration5X(t *testing.T) {
	pacer, err := newPacer("30s@10")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	now := time.Now()
	duration := pacer.waitDuration(now)

	if duration != 0 {
		t.Errorf("duration = %v; want 0", duration)
	}

	now = now.Add(1 * time.Second)
	duration = pacer.waitDuration(now)
	expected := time.Second / 10

	if duration != expected {
		t.Errorf("duration = %v; want %v", duration, expected)
	}
}

func TestPacerDoneOnLastPhaseElapsed(t *testing.T) {
	pacer, err := newPacer("30s@10")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pacer.done {
		t.Errorf("pacer.done = %v; want false", pacer.done)
	}

	pacer.onPhaseElapsed()

	if !pacer.done {
		t.Errorf("pacer.done = %v; want true", pacer.done)
	}
}
