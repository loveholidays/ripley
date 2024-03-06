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
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/metrics"
)

type Options struct {
	Pace                string
	Silent              bool
	SilentHttpError     bool
	DryRun              bool
	Timeout             int
	TimeoutConnection   int
	Strict              bool
	Memprofile          string
	NumWorkers          int
	PrintStat           bool
	MetricsServerEnable bool
	MetricsServerAddr   string
	PrintNSlowest       int
}

// Ensures we have handled all HTTP request results before exiting
var waitGroupResults sync.WaitGroup

func Replay(opts *Options) int {
	// Default exit code
	var exitCode int = 0
	var slowestResults = NewSlowestResults(opts)
	var metricsRequestReceived = make(chan bool)

	// Print metrics and slowest results at the end
	defer printStats(slowestResults, opts)

	// Send requests for the HTTP client workers to pick up on this channel
	var requests = make(chan *Request, opts.NumWorkers)
	defer close(requests)

	// HTTP client workers will send their results on this channel
	var results = make(chan *Result, opts.NumWorkers)
	defer close(results)

	// The pacer controls the rate of replay
	pacer, err := NewPacer(opts.Pace)
	if err != nil {
		panic(err)
	}

	// Read request JSONL input from STDIN
	reader := bufio.NewReaderSize(os.Stdin, 1024*1024)
	scanner := bufio.NewScanner(reader)

	// Start HTTP client goroutine pool
	startClientWorkers(opts, requests, results, slowestResults, metricsRequestReceived)
	pacer.start()

	// Channel to handle interrupt signal
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, syscall.SIGINT, syscall.SIGTERM)

	// Print metrics and slowest results by signal
	go func() {
		<-interruptChan
		printStats(slowestResults, opts)
		os.Exit(exitCode)
	}()

	for scanner.Scan() {
		if pacer.done {
			break
		}
		waitGroupResults.Add(1)

		b := scanner.Bytes()
		req, err := unmarshalRequest(&b)
		if err != nil {
			exitCode = 126
			res := &Result{
				StatusCode: -2,
				Latency:    0,
				Request:    req,
				ErrorMsg:   fmt.Sprintf("%v", err),
			}

			if opts.Strict {
				panic(err)
			}

			results <- res
			continue
		}

		duration := pacer.waitDuration(req.Timestamp)
		time.Sleep(duration)

		requests <- req
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	waitGroupResults.Wait()

	// Wait until the latest Prometheus metrics are received by exporter
	select {
	case <-metricsRequestReceived:
	case <-time.After(time.Duration(2) * time.Second):
	}

	return exitCode
}

func printStats(slowestResults *SlowestResults, opts *Options) {
	if opts.PrintStat {
		metrics.WritePrometheus(os.Stdout, true)
	}
	for _, slowestResult := range slowestResults.results {
		fmt.Println(slowestResult.toJson())
	}
}

func b2s(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
