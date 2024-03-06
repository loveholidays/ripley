package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	rate := flag.Int("rate", 10, "lines per second")
	flag.Parse()

	tickDuration := time.Duration((1000000 / (*rate))) * time.Microsecond

	ticker := time.NewTicker(tickDuration)
	defer ticker.Stop()

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return
	}
	line := scanner.Text()

	for range ticker.C {
		fmt.Println(line)
	}
}
