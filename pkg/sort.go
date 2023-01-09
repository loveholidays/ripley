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
	"container/heap"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type requestSort struct {
	requestHeap *requestHeap
	// Used to check if requests are being emitted out of order
	lastTimestamp time.Time
	exitCode      int
	// Number of requests to keep in the heap - once reached, start popping
	bufferlen int
	// Whether to panic on bad input or out of order timestamps
	strict bool
}

func NewRequestSort(bufferlen int, strict bool) *requestSort {
	rs := &requestSort{}
	rs.requestHeap = &requestHeap{}
	heap.Init(rs.requestHeap)
	rs.lastTimestamp, _ = time.Parse(time.RFC3339, "1987-11-08T18:55:00.9Z")
	rs.exitCode = 0
	rs.bufferlen = bufferlen
	rs.strict = strict
	return rs
}

// An implementation of https://pkg.go.dev/container/heap
// Holds `heapLen` requests sorted by timestamp and prints them in ascending order
type requestHeap []*request

func (rs requestHeap) Less(i, j int) bool {
	return rs[i].Timestamp.Before(rs[j].Timestamp)
}

func (rs requestHeap) Len() int {
	return len(rs)
}

func (rs *requestHeap) Pop() any {
	old := *rs
	n := len(old)
	x := old[n-1]
	*rs = old[0 : n-1]
	return x
}

func (rs *requestHeap) Push(req any) {
	*rs = append(*rs, req.(*request))
}

func (rs requestHeap) Swap(i, j int) {
	rs[i], rs[j] = rs[j], rs[i]
}

func (rs *requestSort) Sort() int {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		req, err := unmarshalRequest(scanner.Bytes())

		if err != nil {
			rs.exitCode = 126
			if rs.strict {
				fmt.Fprintf(os.Stderr, "[%s]\n", scanner.Bytes())
				panic(err)
			}
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		heap.Push(rs.requestHeap, req)

		if rs.bufferlen == 0 {
			rs.popPrint()
		} else {
			rs.bufferlen--
		}
	}

	// Flush the request heap
	for rs.popPrint() {
	}

	return rs.exitCode
}

// Pops a request from the heap and prints it
// Returns false if the heap is empty, true if it isn't
func (rs *requestSort) popPrint() bool {
	r := heap.Pop(rs.requestHeap)

	ts := r.(*request).Timestamp

	// Are we emitting an out of order request?
	if ts.Before(rs.lastTimestamp) {
		errMsg := fmt.Sprintf("Out of order timestamps [%v] and [%v]", rs.lastTimestamp, ts)

		if rs.strict {
			panic(errMsg)
		} else {
			fmt.Fprintln(os.Stderr, errMsg)
			rs.exitCode = 126
		}
	}

	// Remember this timestamp to test the next one we emit isn't before it
	rs.lastTimestamp = ts

	j, err := json.Marshal(r)

	if err != nil {
		if rs.strict {
			panic(err)
		}
		fmt.Fprintln(os.Stderr, err)
		rs.exitCode = 126
	}

	fmt.Println(string(j))

	return rs.requestHeap.Len() > 0
}
