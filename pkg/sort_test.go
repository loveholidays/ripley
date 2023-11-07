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
	"container/heap"
	"testing"
	"time"
)

func TestSort(t *testing.T) {
	t1, _ := time.Parse(time.RFC3339, "2021-11-08T18:55:00.9Z")
	t2, _ := time.Parse(time.RFC3339, "2021-11-08T18:51:00.9Z")
	t3, _ := time.Parse(time.RFC3339, "2021-11-08T18:56:00.9Z")
	r1 := testRequest(t1)
	r2 := testRequest(t2)
	r3 := testRequest(t3)

	reqHeap := &requestHeap{}
	heap.Init(reqHeap)
	heap.Push(reqHeap, r1)
	heap.Push(reqHeap, r2)
	heap.Push(reqHeap, r3)

	r := heap.Pop(reqHeap).(*request)

	if r.Timestamp != t2 {
		t.Errorf("timestamp = %v; want %v", r.Timestamp, t2)
	}

	r = heap.Pop(reqHeap).(*request)

	if r.Timestamp != t1 {
		t.Errorf("timestamp = %v; want %v", r.Timestamp, t1)
	}

	r = heap.Pop(reqHeap).(*request)

	if r.Timestamp != t3 {
		t.Errorf("timestamp = %v; want %v", r.Timestamp, t3)
	}

}

func testRequest(timestamp time.Time) *request {
	return &request{
		"GET",
		"http://example.com",
		"",
		timestamp,
		map[string]string{},
	}
}
