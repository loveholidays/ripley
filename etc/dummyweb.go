package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

var count int

func handler(w http.ResponseWriter, r *http.Request) {
	count++
	// time.Sleep(time.Duration(rand.Intn(250)) * time.Millisecond)
	w.Write([]byte("hi\n"))
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
	log.Fatal(http.ListenAndServe(":8080", nil))
}
