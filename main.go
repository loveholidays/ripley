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

package main

import (
	"flag"
	"fmt"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"

	ripley "github.com/loveholidays/ripley/pkg"
)

func main() {
	var opts ripley.Options

	flag.StringVar(&opts.Pace, "pace", "10s@1", `[duration]@[rate], e.g. "1m@1 30s@1.5 1h@2"`)
	flag.BoolVar(&opts.Silent, "silent", false, "Suppress output")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "Consume input but do not send HTTP requests to targets")
	flag.IntVar(&opts.Timeout, "timeout", 10, "HTTP client request timeout in seconds")
	flag.IntVar(&opts.TimeoutConnection, "timeoutConnection", 3, "HTTP client connetion timeout in seconds")
	flag.BoolVar(&opts.Strict, "strict", false, "Panic on bad input")
	flag.StringVar(&opts.Memprofile, "memprofile", "", "Write memory profile to `file` before exit")
	flag.IntVar(&opts.NumWorkers, "workers", 10, "Number of client workers to use")

	flag.BoolVar(&opts.PrintStat, "printStat", false, "Print statistics to stdout at the end")
	flag.BoolVar(&opts.MetricsServerEnable, "metricsServerEnable", false, "Enable metrics server. Server prometheus statistics on /metrics endpoint")
	flag.StringVar(&opts.MetricsServerAddr, "metricsServerAddr", "0.0.0.0:8081", "Metrics server listen address")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -target string\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	exitCode := ripley.Replay(&opts)
	defer os.Exit(exitCode)

	if opts.Memprofile != "" {
		f, err := os.Create(opts.Memprofile)

		if err != nil {
			panic(err)
		}

		defer f.Close()
		runtime.GC()

		if err := pprof.WriteHeapProfile(f); err != nil {
			panic(err)
		}

		// Wait for a signal to stop the server
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
	}
}
