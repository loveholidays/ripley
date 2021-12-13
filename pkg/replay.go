package ripley

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

func Replay(phasesStr string, silent, printStats bool) {
	// Ensures we have handled all HTTP request results before exiting
	var waitGroup sync.WaitGroup

	// Send requests for the HTTP client workers to pick up on this channel
	requests := make(chan *request)
	defer close(requests)

	// HTTP client workers will send their results on this channel
	results := make(chan *result)
	defer close(results)

	// The pacer controls the rate of replay
	pacer, err := newPacer(phasesStr)

	if err != nil {
		panic(err)
	}

	// If printStats is true, collect and print statistics on exit
	stats := newStats(printStats)

	// Read request JSONL input from STDIN
	scanner := bufio.NewScanner(os.Stdin)

	// Start HTTP client goroutine pool
	startClientWorkers(1000, requests, results)
	pacer.start()

	for scanner.Scan() {
		text := scanner.Text()
		req, err := unmarshalRequest([]byte(text))

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

			if result.err != nil {
				panic(result.err)
			}

			stats.onResult(result)

			jsonResult, err := json.Marshal(result)

			if err != nil {
				panic(err)
			}

			if !silent {
				fmt.Println(string(jsonResult))
			}
		}()
	}

	waitGroup.Wait()
	err = stats.print()

	if err != nil {
		panic(err)
	}
}
