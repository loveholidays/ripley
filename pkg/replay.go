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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

func Replay(phasesStr string, silent, dryRun bool, timeout int, strict bool, maxInFlight, connections, maxConnections int, disableKeepAlives bool, printStatsInterval time.Duration, metricsServerEnable bool, metricsServerAddr string) int {
	return ReplayWithMetricsConfig(phasesStr, silent, dryRun, timeout, strict, maxInFlight, connections, maxConnections, disableKeepAlives, printStatsInterval, MetricsConfig{
		Enabled: metricsServerEnable,
		Address: metricsServerAddr,
	})
}

func ReplayWithMetricsConfig(phasesStr string, silent, dryRun bool, timeout int, strict bool, maxInFlight, connections, maxConnections int, disableKeepAlives bool, printStatsInterval time.Duration, metricsConfig MetricsConfig) int {
	r := &replayer{
		requests: make(chan *Request),
		results:  make(chan *Result),
		metrics:  NewMetricsRecorder(metricsConfig, maxInFlight),
		silent:   silent,
		strict:   strict,
	}
	stopMonitoring := r.metrics.StartMonitoring(r.requests, r.results)
	defer stopMonitoring()

	pacer, err := newPacer(phasesStr)
	if err != nil {
		panic(err)
	}
	pacer.ReportInterval = printStatsInterval
	r.pacer = pacer

	startRequestRunner(maxInFlight, r.requests, r.results, dryRun, timeout, connections, maxConnections, disableKeepAlives)

	handled := make(chan struct{})
	go func() {
		defer close(handled)
		r.handleResults(newLineWriter(os.Stdout))
	}()

	// bufio.Reader.ReadBytes has no line length limit, unlike bufio.Scanner,
	// so large request bodies are fine. The buffer only sets how much is read
	// per call to the OS.
	exitCode := r.sendRequests(bufio.NewReaderSize(os.Stdin, 1<<20))

	// Stop the request runner, wait for every result, then stop the handler.
	close(r.requests)
	r.pending.Wait()
	close(r.results)
	<-handled

	return exitCode
}

// replayer holds the state shared by the two halves of a replay: sending
// requests on schedule, and handling their results.
type replayer struct {
	pacer    *pacer
	metrics  MetricsRecorder
	requests chan *Request // to the request runner
	results  chan *Result  // from the request runner
	pending  sync.WaitGroup
	silent   bool // do not write results
	strict   bool // panic on an invalid request
}

// sendRequests replays each request in input on schedule, until the input or
// the pace ends. It returns the exit code: 126 if any request was invalid.
func (r *replayer) sendRequests(input *bufio.Reader) int {
	exitCode := 0
	for {
		line, ok := nextLine(input)
		if !ok {
			return exitCode
		}
		req, err := unmarshalRequest(line)
		if err != nil {
			exitCode = 126
			r.reject(req, err)
			continue
		}
		if !r.send(req) {
			return exitCode
		}
	}
}

// nextLine returns the next line of input. The line keeps its "\n" or
// "\r\n", which json.Unmarshal treats as whitespace, and a final line without
// one is still returned. It returns false at the end of input and panics if
// the input cannot be read.
func nextLine(input *bufio.Reader) ([]byte, bool) {
	line, err := input.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		panic(err)
	}
	return line, len(line) > 0
}

// reject writes an error result for a request that could not be parsed, and
// panics in strict mode.
func (r *replayer) reject(req *Request, err error) {
	result, _ := json.Marshal(Result{Request: req, ErrorMsg: err.Error()})
	fmt.Println(string(result))
	if r.strict {
		panic(err)
	}
}

// send waits until req is due, then hands it to the request runner. It
// returns false, without sending, once the pace is over.
func (r *replayer) send(req *Request) bool {
	wait, ok := r.pacer.schedule(req.Timestamp)
	if !ok {
		return false
	}
	if req.DisconnectAfter > 0 {
		// The session ends where it ended in the log, mapped onto the wall clock.
		req.disconnectAt = r.pacer.wall(req.Timestamp.Add(time.Duration(req.DisconnectAfter)))
	}
	time.Sleep(wait)
	r.pending.Add(1)
	r.requests <- req
	return true
}

// handleResults records each result and writes it to out, until results is
// closed.
func (r *replayer) handleResults(out *lineWriter) {
	defer func() { _ = out.Flush() }()
	for {
		result, ok := receiveOrFlush(r.results, out)
		if !ok {
			return
		}
		r.pending.Done()

		// If there's a panic elsewhere, this channel can return nil
		if result == nil {
			return
		}
		r.metrics.RecordRequest(result)
		if !r.silent {
			writeResult(out, result)
		}
	}
}

func writeResult(out *lineWriter, result *Result) {
	data, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	_ = out.WriteLine(append(data, '\n'))
}
