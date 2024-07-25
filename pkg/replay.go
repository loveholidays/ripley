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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

func Replay(phasesStr string, silent, dryRun bool, timeout int, strict bool, numWorkers, connections, maxConnections int) int {
	// Default exit code
	var exitCode = 0
	// Ensures we have handled all HTTP request results before exiting
	var waitGroup sync.WaitGroup

	// Send requests for the HTTP client workers to pick up on this channel
	requests := make(chan *request)
	defer close(requests)

	// HTTP client workers will send their results on this channel
	results := make(chan *Result)
	defer close(results)

	// Read request JSONL input from STDIN
	scanner := bufio.NewScanner(bufio.NewReaderSize(os.Stdin, 32*1024*1024))

	// Start HTTP client goroutine pool
	startClientWorkers(numWorkers, requests, results, dryRun, timeout, connections, maxConnections)

	// Goroutine to handle the  HTTP client result
	go func() {
		for result := range results {
			r := result
			go func() {
				defer waitGroup.Done()
				// If there's a panic elsewhere, this channel can return nil
				if r == nil {
					return
				}
				if !silent {
					jsonResult, err := json.Marshal(r)
					if err != nil {
						panic(err)
					}
					fmt.Println(string(jsonResult))
				}
			}()
		}
	}()

	// The pacer controls the rate of replay
	pacer, err := NewPhaseTicker(context.TODO(), phasesStr)
	if err != nil {
		panic(err)
	}

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

		<-pacer
		waitGroup.Add(1)
		requests <- req
	}

	if scanner.Err() != nil {
		panic(scanner.Err())
	}

	waitGroup.Wait()

	return exitCode
}
