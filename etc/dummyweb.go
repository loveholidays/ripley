package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"
)

// Counters for the once-a-second report. Atomic, because every request runs
// in its own goroutine.
var (
	requests atomic.Int64 // requests to / this second
	streams  atomic.Int64 // /sse streams started this second
	open     atomic.Int64 // /sse streams open now
)

func handler(w http.ResponseWriter, r *http.Request) {
	requests.Add(1)
	time.Sleep(time.Duration(rand.Intn(250)) * time.Millisecond)
	_, _ = w.Write([]byte("hi\n"))
}

// sseHandler streams an event every 100ms until the client disconnects. With
// ?lifetime=10s, the server instead ends the stream cleanly after that long.
func sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	var end <-chan time.Time // nil never fires: stream until the client leaves
	if s := r.URL.Query().Get("lifetime"); s != "" {
		lifetime, err := time.ParseDuration(s)
		if err != nil || lifetime <= 0 {
			http.Error(w, "lifetime must be a positive duration such as 10s", http.StatusBadRequest)
			return
		}
		end = time.After(lifetime)
	}

	streams.Add(1)
	open.Add(1)
	defer open.Add(-1)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher.Flush()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for id := 0; ; id++ {
		select {
		case <-r.Context().Done():
			return
		case <-end:
			return
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, "id: %d\ndata: event %d\n\n", id, id); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func main() {
	// Crude per-second reporting: "rps" counts requests to /, "sse/s" new
	// /sse streams, and "sse open" the streams still open.
	go func() {
		for range time.Tick(time.Second) {
			fmt.Printf("rps: %d sse/s: %d sse open: %d\n", requests.Swap(0), streams.Swap(0), open.Load())
		}
	}()

	http.HandleFunc("/", handler)
	http.HandleFunc("/sse", sseHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
