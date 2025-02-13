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

package main

import (
	"flag"
	"os"
	"runtime"
	"runtime/pprof"

	ripley "github.com/loveholidays/ripley/pkg"
)

func main() {
	exitCode := 0

	paceStr := flag.String("pace", "10s@1", `[duration]@[rate], e.g. "1m@1 30s@1.5 1h@2"`)
	silent := flag.Bool("silent", false, "Suppress output")
	dryRun := flag.Bool("dry-run", false, "Consume input but do not send HTTP requests to targets")
	timeout := flag.Int("timeout", 10, "HTTP client timeout in seconds")
	connections := flag.Int("connections", 10000, "Max open idle connections per target host")
	maxConnections := flag.Int("max-connections", 0, "Max connections per target host (default unlimited)")
	strict := flag.Bool("strict", false, "Panic on bad input")
	memprofile := flag.String("memprofile", "", "Write memory profile to `file` before exit")
	cpuprofile := flag.String("cpuprofile", "", "Write cpu profile to `file` before exit")
	numWorkers := flag.Int("workers", runtime.NumCPU()*2, "Number of client workers to use")
	printStatsInterval := flag.Duration("print-stats", 0, `Statistics report interval, e.g., "1m"

Each report line is printed to stderr with the following fields in logfmt format:

  report_time
    The calculated wall time for when this line should be printed in RFC3339 format.

  skew_seconds
    Difference between "report_time" and current time in seconds. When the absolute
    value of this is higher than about 100ms, it shows that ripley cannot generate
    enough load. Consider increasing workers, max connections, and/or CPU and IO requests.

  last_request_time
    Original request time of the last request in RFC3339 format.

  rate
    Current rate of playback as specified in "pace" flag.

  expected_rps
    Expected requests per second since the last report. This will differ from the
    actual requests per second if the system is unable to drive that many requests.
    If that is the case, consider increasing workers, max connections, and/or
    CPU and IO requests.

When 0 (default) or negative, reporting is switched off.
  `)

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)

		if err != nil {
			panic(err)
		}

		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			panic(err)
		}

		defer pprof.StopCPUProfile()
	}

	exitCode = ripley.Replay(*paceStr, *silent, *dryRun, *timeout, *strict, *numWorkers, *connections, *maxConnections, *printStatsInterval)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)

		if err != nil {
			panic(err)
		}

		defer f.Close()
		runtime.GC()

		if err := pprof.WriteHeapProfile(f); err != nil {
			panic(err)
		}
	}

	os.Exit(exitCode)
}
