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
)

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

// `bufferlen` is the number of requests to keep in the heap - once reached, start popping
func Sort(bufferlen int, strict bool) int {
	exitCode := 0
	reqHeap := &requestHeap{}
	heap.Init(reqHeap)
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		req, err := unmarshalRequest(scanner.Bytes())

		if err != nil {
			exitCode = 126
			if strict {
				panic(err)
			}
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		heap.Push(reqHeap, req)

		if bufferlen == 0 {
			exitCode = popPrint(reqHeap, strict)
		} else {
			bufferlen--
		}
	}

	for {
		if reqHeap.Len() == 0 {
			break
		}

		exitCode = popPrint(reqHeap, strict)

	}

	return exitCode
}

func popPrint(reqHeap *requestHeap, strict bool) int {
	r := heap.Pop(reqHeap)
	j, err := json.Marshal(r)

	if err != nil {
		if strict {
			panic(err)
		}
		fmt.Fprintln(os.Stderr, err)
		return 126
	}

	fmt.Println(string(j))
	return 0
}
