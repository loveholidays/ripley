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
	"fmt"
	"os"
	"sync"
	"time"
)

func Replay(phasesStr string, silent, dryRun bool, timeout int, strict bool, numWorkers, connections, maxConnections int, printStatsInterval time.Duration, metricsServerEnable bool, metricsServerAddr string) int {
	// Default exit code
	var exitCode = 0
	// Ensures we have handled all HTTP Request results before exiting
	var waitGroup sync.WaitGroup
	// Ensures result handler goroutine completes before closing results channel
	var resultHandlerWG sync.WaitGroup

	// Send requests for the HTTP client workers to pick up on this channel
	requests := make(chan *Request)

	// HTTP client workers will send their results on this channel
	results := make(chan *Result)

	// Initialize metrics recorder (no-op if disabled)
	metricsRecorder := NewMetricsRecorder(MetricsConfig{
		Enabled: metricsServerEnable,
		Address: metricsServerAddr,
	}, numWorkers)
	stopMonitoring := metricsRecorder.StartMonitoring(requests, results)
	defer stopMonitoring()

	// The pacer controls the rate of replay
	pacer, err := newPacer(phasesStr)
	pacer.ReportInterval = printStatsInterval

	if err != nil {
		panic(err)
	}

	// Read Request JSONL input from STDIN
	scanner := bufio.NewScanner(bufio.NewReaderSize(os.Stdin, 32*1024*1024))

	// Start HTTP client goroutine pool
	startClientWorkers(numWorkers, requests, results, dryRun, timeout, connections, maxConnections)
	pacer.start()

	// Goroutine to handle the  HTTP client result
	resultHandlerWG.Add(1)
	go func() {
		defer resultHandlerWG.Done()
		for result := range results {
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

				fmt.Println(string(jsonResult))
			}
		}
	}()

	for scanner.Scan() {
		req, err := unmarshalRequest(scanner.Bytes())
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

		if pacer.isDone() {
			break
		}

		// The pacer decides how long to wait between requests
		waitDuration := pacer.waitDuration(req.Timestamp)
		time.Sleep(waitDuration)
		waitGroup.Add(1)
		requests <- req
	}

	if scanner.Err() != nil {
		panic(scanner.Err())
	}

	// Close requests channel to signal worker goroutines to stop
	close(requests)

	// Wait for all HTTP requests to complete
	waitGroup.Wait()

	// Close results channel to signal result handler to stop
	close(results)

	// Wait for result handler to finish processing all results
	resultHandlerWG.Wait()

	return exitCode
}
