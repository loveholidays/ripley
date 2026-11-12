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

// Package slowio provides readers that simulate slow network clients.
package slowio

import (
	"context"
	"io"
	"time"
)

// chunkInterval is the most data, in time at the target rate, that one Read
// returns before sleeping. Longer means fewer wake-ups across many slow
// streams; shorter means smoother pacing.
const chunkInterval = 250 * time.Millisecond

// NewReader returns a reader that reads from r at an average of at most rate
// bytes per second. rate must be positive. Each Read returns at most
// chunkInterval's worth of bytes, then sleeps until they are due, so the limit
// is an average with small bursts. Bytes returned together with an error are
// paced too.
//
// When r is slower than rate, Read takes data as it arrives, and the time spent
// waiting earns at most chunkInterval of allowance: a burst after a quiet
// period is still paced at rate, apart from one chunk. This follows Chrome's
// DevTools throttling, and matters for streams such as SSE that are often
// silent.
//
// The reader holds ctx because io.Reader has no other way to receive one: if
// ctx ends while Read is sleeping, Read returns the bytes it read and
// ctx.Err().
//
// NewReader panics if rate is not positive.
func NewReader(ctx context.Context, r io.Reader, rate int64) io.Reader {
	if rate <= 0 {
		panic("slowio: non-positive rate for NewReader")
	}
	return &reader{
		ctx:   ctx,
		src:   r,
		rate:  rate,
		chunk: int(max(rate*int64(chunkInterval)/int64(time.Second), 1)),
		next:  time.Now(), // no allowance before the first byte
	}
}

// reader paces reads from src. It reuses one timer, so a long stream does not
// allocate per wake-up; an unstopped timer is garbage collected since Go 1.23.
type reader struct {
	ctx   context.Context
	src   io.Reader
	rate  int64     // bytes per second
	chunk int       // most bytes per Read; matters when smaller than the caller's buffer
	next  time.Time // when the bytes read so far are due
	timer *time.Timer
}

func (r *reader) Read(p []byte) (int, error) {
	if len(p) > r.chunk {
		p = p[:r.chunk]
	}
	n, err := r.src.Read(p)
	if waitErr := r.wait(n); waitErr != nil {
		return n, waitErr
	}
	return n, err
}

// wait schedules n bytes at rate and sleeps until they are due, or until ctx
// ends. They follow the previous bytes, so time lost to late timer wake-ups is
// made up. But the schedule may lag now by at most chunkInterval: more than
// that means src was slower than rate, and that idle time must not earn
// allowance.
func (r *reader) wait(n int) error {
	now := time.Now()
	if earliest := now.Add(-chunkInterval); r.next.Before(earliest) {
		r.next = earliest
	}
	r.next = r.next.Add(time.Duration(n) * time.Second / time.Duration(r.rate))
	d := r.next.Sub(now)
	if d <= 0 { // n was 0, or catching up within the allowance
		return nil
	}
	if r.timer == nil {
		r.timer = time.NewTimer(d)
	} else {
		r.timer.Reset(d) // Safe without draining since Go 1.23.
	}
	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	case <-r.timer.C:
		return nil
	}
}
