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
	"encoding/json"
	"fmt"
	"github.com/montanaflynn/stats"
)

type statistics struct {
	active      bool
	latencies   []float64
	statusCodes map[int]int
}

type report struct {
	TotalRequests int                `json:"totalRequests"`
	StatusCodes   map[int]int        `json:"statusCodes"`
	Latency       map[string]float64 `json:"latencyMicroseconds"`
}

func newStats(active bool) *statistics {
	statusCodes := make(map[int]int)
	latencies := make([]float64, 0)
	return &statistics{active, latencies, statusCodes}
}

func (s *statistics) onResult(result *result) {
	if s.active {
		s.latencies = append(s.latencies, float64(result.Latency.Microseconds()))

		_, ok := s.statusCodes[result.StatusCode]

		if !ok {
			s.statusCodes[result.StatusCode] = 0
		}

		s.statusCodes[result.StatusCode]++
	}
}

func (s *statistics) print() error {
	if s.active {
		min, err := stats.Min(s.latencies)

		if err != nil {
			return err
		}

		mean, err := stats.Mean(s.latencies)

		if err != nil {
			return err
		}

		median, err := stats.Median(s.latencies)

		if err != nil {
			return err
		}

		max, err := stats.Max(s.latencies)

		if err != nil {
			return err
		}

		p95, err := stats.Percentile(s.latencies, 95.0)

		if err != nil {
			return err
		}

		p99, err := stats.Percentile(s.latencies, 99.0)

		if err != nil {
			return err
		}

		stdDev, err := stats.StandardDeviation(s.latencies)

		if err != nil {
			return err
		}

		report := &report{}
		report.TotalRequests = len(s.latencies)
		report.StatusCodes = s.statusCodes
		report.Latency = make(map[string]float64)
		report.Latency["min"] = min
		report.Latency["mean"] = mean
		report.Latency["median"] = median
		report.Latency["p95"] = p95
		report.Latency["p99"] = p99
		report.Latency["max"] = max
		report.Latency["stdDev"] = stdDev

		jsonReport, err := json.Marshal(report)

		if err != nil {
			return err
		}

		fmt.Println(string(jsonReport))
	}

	return nil
}
