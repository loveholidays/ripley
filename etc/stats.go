// An example on how to interpret and analyse ripley output
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/loveholidays/ripley/pkg"
	"github.com/montanaflynn/stats"
)

type report struct {
	TotalRequests int                `json:"totalRequests"`
	StatusCodes   map[int]int        `json:"statusCodes"`
	Latency       map[string]float64 `json:"latency"`
}

func main() {
	statusCodes := make(map[int]int)
	latencies := make([]float64, 0)

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		result := &ripley.Result{}
		err := json.Unmarshal(scanner.Bytes(), &result)

		if err != nil {
			panic(err)
		}

		_, ok := statusCodes[result.StatusCode]

		if !ok {
			statusCodes[result.StatusCode] = 0
		}

		statusCodes[result.StatusCode]++
		latency := float64(result.Latency.Nanoseconds())
		latencies = append(latencies, latency)
	}

	if scanner.Err() != nil {
		panic(scanner.Err())
	}

	min, err := stats.Min(latencies)

	if err != nil {
		panic(err)
	}

	mean, err := stats.Mean(latencies)

	if err != nil {
		panic(err)
	}

	median, err := stats.Median(latencies)

	if err != nil {
		panic(err)
	}

	max, err := stats.Max(latencies)

	if err != nil {
		panic(err)
	}

	p95, err := stats.Percentile(latencies, 95.0)

	if err != nil {
		panic(err)
	}

	p99, err := stats.Percentile(latencies, 99.0)

	if err != nil {
		panic(err)
	}

	stdDev, err := stats.StandardDeviation(latencies)

	if err != nil {
		panic(err)
	}

	report := &report{Latency: make(map[string]float64)}
	report.TotalRequests = len(latencies)
	report.StatusCodes = statusCodes
	report.Latency["min"] = min
	report.Latency["mean"] = mean
	report.Latency["median"] = median
	report.Latency["p95"] = p95
	report.Latency["p99"] = p99
	report.Latency["max"] = max
	report.Latency["stdDev"] = stdDev

	reportJson, err := json.Marshal(report)

	if err != nil {
		panic(err)
	}

	fmt.Println(string(reportJson))
}
