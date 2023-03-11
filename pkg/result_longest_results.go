package ripley

import (
	"container/heap"
	"sync"
)

type ResultHeap struct {
	results []*Result
	lock    sync.Mutex
	once    sync.Once
}

var longestResultsHeap = &ResultHeap{}

func (h *ResultHeap) Len() int {
	return len(h.results)
}

func (h *ResultHeap) Less(i, j int) bool {
	return h.results[i].Latency > h.results[j].Latency
}

func (h *ResultHeap) Swap(i, j int) {
	h.results[i], h.results[j] = h.results[j], h.results[i]
}

func (h *ResultHeap) Push(x interface{}) {
	h.results = append(h.results, x.(*Result))
}

func (h *ResultHeap) Pop() interface{} {
	old := h.results
	n := len(old)
	x := old[n-1]
	h.results = old[:n-1]
	return x
}

func (h *ResultHeap) getOrCreateLongestResultsHeap() *ResultHeap {
	h.once.Do(func() {
		heap.Init(longestResultsHeap)
	})

	return longestResultsHeap
}

func storeLongestResults(result *Result, opts *Options) {
	if !opts.NlongestPrint {
		return
	}

	h := longestResultsHeap.getOrCreateLongestResultsHeap()

	h.lock.Lock()
	defer h.lock.Unlock()

	heap.Push(h, result)
	if h.Len() > opts.NlongestResults {
		heap.Pop(h)
	}
}
