package main

import (
	"log"
	"net/http"
	"time"
)

var count int

func handler(w http.ResponseWriter, r *http.Request) {
	count++
	w.Write([]byte("hi\n"))
}

func main() {
	// Crude RPS reporting
	go func() {
		for {
			println(count)
			count = 0
			time.Sleep(time.Second)
		}
	}()

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
