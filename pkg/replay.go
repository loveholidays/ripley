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

func Replay(phasesStr string, silent, dryRun bool, timeout int) {
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

	if err != nil {
		panic(err)
	}

	// Read request JSONL input from STDIN
	scanner := bufio.NewScanner(os.Stdin)

	// Start HTTP client goroutine pool
	startClientWorkers(1000, requests, results, dryRun, timeout)
	pacer.start()

	for scanner.Scan() {
		req, err := unmarshalRequest(scanner.Bytes())

		if err != nil {
			panic(err)
		}

		if pacer.done {
			break
		}

		// The pacer decides how long to wait between requests
		waitDuration := pacer.waitDuration(req.Timestamp)
		time.Sleep(waitDuration)
		requests <- req
		waitGroup.Add(1)

		// Goroutine to handle the  HTTP client result
		go func() {
			defer waitGroup.Done()

			result := <-results

			// If there's a panic elsewhere, this channel can return nil
			if result == nil {
				return
			}

			jsonResult, err := json.Marshal(result)

			if err != nil {
				panic(err)
			}

			if !silent {
				fmt.Println(string(jsonResult))
			}
		}()
	}

	if scanner.Err() != nil {
		panic(scanner.Err())
	}

	waitGroup.Wait()
}
