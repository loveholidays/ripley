package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

var count int

func handler(w http.ResponseWriter, r *http.Request) {
	count++
	time.Sleep(time.Duration(rand.Intn(250)) * time.Millisecond)
	_, _ = w.Write([]byte("hi\n"))
}

// sseHandler streams an event every 100ms until the client disconnects.
func sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher.Flush()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for id := 0; ; id++ {
		select {
		case <-r.Context().Done():
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
	// Crude RPS reporting
	go func() {
		for {
			fmt.Printf("rps: %d\n", count)
			count = 0
			time.Sleep(time.Second)
		}
	}()

	http.HandleFunc("/", handler)
	http.HandleFunc("/sse", sseHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
