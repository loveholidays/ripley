package ripley

import "sort"

type SlowestResults struct {
	results         []Result
	nSlowestResults int
}

func (h *SlowestResults) store(result *Result) {
	if h.nSlowestResults == 0 {
		return
	}

	h.results = append(h.results, *result)
	if len(h.results) > h.nSlowestResults {
		sort.Slice(h.results, func(i, j int) bool {
			return h.results[i].Latency > h.results[j].Latency
		})
		h.results = h.results[:h.nSlowestResults]
	}
}

func NewSlowestResults(opts *Options) *SlowestResults {
	return &SlowestResults{
		nSlowestResults: opts.PrintNSlowest,
		results:         make([]Result, 0, opts.PrintNSlowest),
	}
}
