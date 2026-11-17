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
	"bufio"
	"io"
)

// lineWriter batches lines for w, so a busy replay is not held back by one
// write call per result. It writes only whole lines, so other writers to the
// same file, such as input error reports, never split one of its lines.
type lineWriter struct {
	buf *bufio.Writer
}

func newLineWriter(w io.Writer) *lineWriter {
	return &lineWriter{buf: bufio.NewWriterSize(w, 64<<10)}
}

// WriteLine adds line, which must end in a newline, to the batch.
func (lw *lineWriter) WriteLine(line []byte) error {
	// Flush first rather than let bufio split the line across two writes. A
	// line longer than the buffer then goes to w in a single write.
	if len(line) > lw.buf.Available() && lw.buf.Buffered() > 0 {
		if err := lw.buf.Flush(); err != nil {
			return err
		}
	}
	_, err := lw.buf.Write(line)
	return err
}

// Flush writes the batch to w.
func (lw *lineWriter) Flush() error {
	return lw.buf.Flush()
}

// receiveOrFlush receives the next result. When none is waiting, it flushes
// out before blocking, so output is held back only while more results are
// ready to be written.
func receiveOrFlush(results <-chan *Result, out *lineWriter) (*Result, bool) {
	select {
	case result, ok := <-results:
		return result, ok
	default:
	}
	_ = out.Flush() // as before, a failed write to stdout does not stop the replay
	result, ok := <-results
	return result, ok
}
