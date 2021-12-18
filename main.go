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
	"github.com/loveholidays/ripley/pkg"
)

func main() {
	paceStr := flag.String("pace", "10s@1", `[duration]@[rate], e.g. "1m@1 30s@1.5 1h@2"`)
	silent := flag.Bool("silent", false, "Suppress output")
	printStats := flag.Bool("stats", false, "Collect and print statistics before the program exits")

	flag.Parse()

	ripley.Replay(*paceStr, *silent, *printStats)
}
