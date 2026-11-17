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
	// Default exit code
	var exitCode = 0
	// Ensures we have handled all HTTP Request results before exiting
	var waitGroup sync.WaitGroup
	// Ensures result handler goroutine completes before closing results channel
	var resultHandlerWG sync.WaitGroup

	// Send requests to the request runner on this channel
	requests := make(chan *Request)

	// Requests send their results on this channel
	results := make(chan *Result)

	// Initialize metrics recorder (no-op if disabled)
	metricsRecorder := NewMetricsRecorder(metricsConfig, maxInFlight)
	stopMonitoring := metricsRecorder.StartMonitoring(requests, results)
	defer stopMonitoring()

	// The pacer controls the rate of replay
	pacer, err := newPacer(phasesStr)
	if err != nil {
		panic(err)
	}
	pacer.ReportInterval = printStatsInterval

	// Read Request JSONL input from STDIN. bufio.Reader.ReadBytes has no line
	// length limit, unlike bufio.Scanner, so large request bodies are fine.
	// The buffer only sets how much is read per call to the OS.
	input := bufio.NewReaderSize(os.Stdin, 1<<20)

	// Start running requests as they arrive
	startRequestRunner(maxInFlight, requests, results, dryRun, timeout, connections, maxConnections, disableKeepAlives)

	// Goroutine to handle the  HTTP client result
	resultHandlerWG.Add(1)
	go func() {
		defer resultHandlerWG.Done()
		out := newLineWriter(os.Stdout)
		defer func() { _ = out.Flush() }()
		for {
			result, ok := receiveOrFlush(results, out)
			if !ok {
				return
			}
			waitGroup.Done()

			// If there's a panic elsewhere, this channel can return nil
			if result == nil {
				return
			}

			metricsRecorder.RecordRequest(result)

			if !silent {
				jsonResult, err := json.Marshal(result)

				if err != nil {
					panic(err)
				}

				_ = out.WriteLine(append(jsonResult, '\n'))
			}
		}
	}()

	for {
		// The line keeps its "\n" or "\r\n"; json.Unmarshal treats it as whitespace.
		line, readErr := input.ReadBytes('\n')
		if errors.Is(readErr, io.EOF) && len(line) == 0 {
			break
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			panic(readErr)
		}

		req, err := unmarshalRequest(line)
		if err != nil {
			exitCode = 126
			result, _ := json.Marshal(Result{
				StatusCode: 0,
				Latency:    0,
				Request:    req,
				ErrorMsg:   fmt.Sprintf("%v", err),
			})
			fmt.Println(string(result))

			if strict {
				panic(err)
			}
			continue
		}

		// The pacer decides when each request is sent
		wait, ok := pacer.schedule(req.Timestamp)
		if !ok {
			break
		}
		if req.DisconnectAfter > 0 {
			// The session ends where it ended in the log, mapped onto the wall clock.
			req.disconnectAt = pacer.wall(req.Timestamp.Add(time.Duration(req.DisconnectAfter)))
		}
		time.Sleep(wait)
		waitGroup.Add(1)
		requests <- req
	}

	// Close requests channel to signal the request runner to stop
	close(requests)

	// Wait for all HTTP requests to complete
	waitGroup.Wait()

	// Close results channel to signal result handler to stop
	close(results)

	// Wait for result handler to finish processing all results
	resultHandlerWG.Wait()

	return exitCode
}
