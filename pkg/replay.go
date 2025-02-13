/*
ripley
Copyright (C) 2021  loveholidays

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU Lesser General Public
License as published by the Free Software Foundation; either
version 3 of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
Lesser General Public License for more details.

You should have received a copy of the GNU Lesser General Public License
along with this program; if not, write to the Free Software Foundation,
Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
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

func Replay(phasesStr string, silent, dryRun bool, timeout int, strict bool, numWorkers, connections, maxConnections int, printStatsInterval time.Duration) int {
	// Default exit code
	var exitCode int = 0
	// Ensures we have handled all HTTP request results before exiting
	var waitGroup sync.WaitGroup

	// Send requests for the HTTP client workers to pick up on this channel
	requests := make(chan *request)
	defer close(requests)

	// HTTP client workers will send their results on this channel
	results := make(chan *Result)
	defer close(results)

	// The pacer controls the rate of replay
	pacer, err := newPacer(phasesStr)
	pacer.ReportInterval = printStatsInterval

	if err != nil {
		panic(err)
	}

	// Read request JSONL input from STDIN
	scanner := bufio.NewScanner(bufio.NewReaderSize(os.Stdin, 32*1024*1024))

	// Start HTTP client goroutine pool
	startClientWorkers(numWorkers, requests, results, dryRun, timeout, connections, maxConnections)
	pacer.start()

	// Goroutine to handle the  HTTP client result
	go func() {
		for result := range results {
			waitGroup.Done()

			// If there's a panic elsewhere, this channel can return nil
			if result == nil {
				return
			}

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

		if pacer.done {
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

	waitGroup.Wait()

	return exitCode
}
