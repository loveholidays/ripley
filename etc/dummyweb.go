package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"golang.org/x/net/netutil"
)

var count atomic.Int64
var d = []byte("hi\n")

func handler(w http.ResponseWriter, r *http.Request) {
	count.Add(1)
	time.Sleep(time.Duration(rand.Intn(250)) * time.Millisecond)
	w.Write(d)
}

func main() {
	// Crude RPS reporting
	go func() {
		for {
			fmt.Printf("rps: %d\n", count.Swap(0))
			time.Sleep(time.Second)
		}
	}()
	listener, _ := net.Listen("tcp", ":8080")

	limitedListener := netutil.LimitListener(listener, 20000) // Limit to 10 connections

	http.HandleFunc("/", handler)
	log.Fatal(http.Serve(limitedListener, nil))
}
