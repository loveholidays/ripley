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

// Ensures we have handled all HTTP request results before exiting
var waitGroupResults sync.WaitGroup

func Replay(phasesStr string, silent, dryRun bool, timeout int, strict bool, numWorkers int) int {
	// Default exit code
	var exitCode int = 0
	// Minimal internal jitter
	const jitter = 5 * time.Millisecond

	// Send requests for the HTTP client workers to pick up on this channel
	requests := make(chan *request, numWorkers)
	defer close(requests)

	// HTTP client workers will send their results on this channel
	results := make(chan *Result, numWorkers)
	defer close(results)

	// The pacer controls the rate of replay
	pacer, err := newPacer(phasesStr)
	if err != nil {
		panic(err)
	}

	// Read request JSONL input from STDIN
	reader := bufio.NewReaderSize(os.Stdin, 1024*1024)
	scanner := bufio.NewScanner(reader)

	// Start HTTP client goroutine pool
	startClientWorkers(numWorkers, requests, results, dryRun, timeout, silent)
	pacer.start()

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

		waitGroupResults.Add(1)

		duration := pacer.waitDuration(req.Timestamp)
		// This is meant to utilize the delay that occurs internally when deserializing/parsing/etc,
		// as well as any minor internal fluctuations.
		if duration >= jitter {
			time.Sleep(duration)
		}

		// curRLen := len(requests)
		// if curRLen > 0 && curRLen >= prevRLen {
		// 	fmt.Printf("Queue is overflow. Consider to increase workers or slow down request rate. Requests %d\n", curRLen)
		// }
		// prevRLen = curRLen

		requests <- req
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	waitGroupResults.Wait()
	return exitCode
}
