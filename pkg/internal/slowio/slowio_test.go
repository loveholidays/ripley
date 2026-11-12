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

package slowio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestReaderPaces(t *testing.T) {
	start := time.Now()
	r := NewReader(context.Background(), bytes.NewReader(make([]byte, 100)), 1000)
	if n, err := io.Copy(io.Discard, r); n != 100 || err != nil {
		t.Fatalf("Copy = %d, %v; want 100, nil", n, err)
	}
	if took := time.Since(start); took < 80*time.Millisecond || took > 2*time.Second {
		t.Errorf("100 bytes at 1000 B/s took %v; want about 100ms", took)
	}
}

func TestReadReturnsAtMostAChunk(t *testing.T) {
	r := NewReader(context.Background(), bytes.NewReader(make([]byte, 1000)), 1000)
	if n, err := r.Read(make([]byte, 1000)); n != 250 || err != nil { // 250ms at 1000 B/s
		t.Fatalf("Read = %d, %v; want 250, nil", n, err)
	}
}

// finalReader returns its only byte together with err, as io.Reader allows.
type finalReader struct{ err error }

func (r finalReader) Read(p []byte) (int, error) {
	p[0] = 'x'
	return 1, r.err
}

func TestReaderPacesBytesReturnedWithEOF(t *testing.T) {
	start := time.Now()
	n, err := NewReader(context.Background(), finalReader{io.EOF}, 20).Read(make([]byte, 8))
	if n != 1 || err != io.EOF {
		t.Fatalf("Read = %d, %v; want 1, EOF", n, err)
	}
	if took := time.Since(start); took < 40*time.Millisecond {
		t.Errorf("1 byte at 20 B/s took %v; want about 50ms", took)
	}
}

func TestReaderReturnsReadError(t *testing.T) {
	_, err := NewReader(context.Background(), finalReader{io.ErrUnexpectedEOF}, 1000).Read(make([]byte, 8))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v; want unexpected EOF", err)
	}
}

func TestReaderStopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	n, err := NewReader(ctx, bytes.NewReader(make([]byte, 10)), 1).Read(make([]byte, 8))
	if n != 1 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Read = %d, %v; want 1, deadline exceeded", n, err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("returned after %v; want promptly after the context ended", took)
	}
}

// lateReader is silent for delay on its first Read, then serves r.
type lateReader struct {
	r       io.Reader
	delay   time.Duration
	started bool
}

func (l *lateReader) Read(p []byte) (int, error) {
	if !l.started {
		l.started = true
		time.Sleep(l.delay)
	}
	return l.r.Read(p)
}

func TestSilenceEarnsAtMostOneChunk(t *testing.T) {
	// Silent for a second, then 1000 bytes at once. At 1000 B/s, after the
	// first chunk the rest must still take about 750ms: the quiet period earns
	// at most one chunk of allowance. Banking it all would read the rest at once.
	src := &lateReader{r: bytes.NewReader(make([]byte, 1000)), delay: time.Second}
	r := NewReader(context.Background(), src, 1000)
	buf := make([]byte, 1000)
	n, err := r.Read(buf) // waits out the silence, then reads the first chunk
	burst := time.Now()
	if err != nil {
		t.Fatal(err)
	}
	for n < 1000 {
		m, err := r.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		n += m
	}
	if took := time.Since(burst); took < 650*time.Millisecond {
		t.Errorf("rest of the burst took %v; want about 750ms, not sped up by the silence", took)
	}
}

func TestNewReaderPanicsOnNonPositiveRate(t *testing.T) {
	for _, rate := range []int64{0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("NewReader with rate %d did not panic", rate)
				}
			}()
			NewReader(context.Background(), bytes.NewReader(nil), rate)
		}()
	}
}

// zeros is an endless source of zero bytes.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestReaderKeepsRateDespiteLateWakeups(t *testing.T) {
	// Timers wake slightly late. At fast-4g speed, with io.Discard's 8 KiB
	// reads, that is about 120 wake-ups a second, and losing the lateness each
	// time would make a second of data take about 10% longer.
	const rate = 1_012_500
	start := time.Now()
	r := io.LimitReader(NewReader(context.Background(), zeros{}, rate), rate)
	if _, err := io.Copy(io.Discard, r); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 950*time.Millisecond || took > 1060*time.Millisecond {
		t.Errorf("1s of data at %d B/s took %v; want about 1s", rate, took)
	}
}

func TestReadDoesNotAllocate(t *testing.T) {
	r := NewReader(context.Background(), zeros{}, 1_000_000) // 1 byte per µs
	buf := make([]byte, 100)
	_, _ = r.Read(buf) // creates the timer
	allocs := testing.AllocsPerRun(100, func() {
		if _, err := r.Read(buf); err != nil { // sleeps about 100µs
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("Read allocated %v times per call; want 0", allocs)
	}
}
