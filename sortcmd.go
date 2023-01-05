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

// Build: `go build -o ripleysort sortcmd.go`
// Run: cat requests.jsonl | ./ripleysort -bufferlen 1000 | jq '.["timestamp"]'
package main

import (
	"flag"
	"os"

	ripley "github.com/loveholidays/ripley/pkg"
)

func main() {
	bufferlen := flag.Int("bufferlen", 100, "Number of requests to keep in memory")
	strict := flag.Bool("strict", false, "Panic on bad input")
	flag.Parse()
	os.Exit(ripley.Sort(*bufferlen, *strict))
}
