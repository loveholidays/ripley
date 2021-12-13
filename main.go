package main

import (
	"flag"
	"github.com/loveholidays/ripley/pkg"
)

func main() {
	paceStr := flag.String("pace", "10s@1", `[duration]@[rate], e.g. "1m@1 30s@1.5 1h@2"`)
	silent := flag.Bool("silent", false, "Suppress output")
	printStats := flag.Bool("stats", false, "Collect and print statistics before the program exits")

	flag.Parse()

	ripley.Replay(*paceStr, *silent, *printStats)
}
