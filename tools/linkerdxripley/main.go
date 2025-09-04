package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/loveholidays/ripley/tools/linkerdxripley/pkg/converter"
	"github.com/loveholidays/ripley/tools/linkerdxripley/pkg/linkerd"
)

func main() {
	var (
		newHost = flag.String("host", "", "New host to replace the original host in URLs")
		https   = flag.Bool("https", false, "Upgrade HTTP requests to HTTPS")
		help    = flag.Bool("help", false, "Show usage information")
	)
	flag.Parse()

	if *help {
		fmt.Fprintf(os.Stderr, "linkerdxripley - Convert Linkerd JSONL format to Ripley format\n\n")
		fmt.Fprintf(os.Stderr, "Usage: linkerdxripley [options] < input.jsonl > output.jsonl\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  cat linkerd.jsonl | linkerdxripley -host localhost:8080 > ripley.jsonl\n")
		fmt.Fprintf(os.Stderr, "  cat linkerd.jsonl | linkerdxripley -host localhost:8080 -https > ripley.jsonl\n\n")
		return
	}

	conv := converter.New()
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var linkerdReq linkerd.Request
		if err := json.Unmarshal([]byte(line), &linkerdReq); err != nil {
			log.Printf("failed to parse line: %s, error: %v", line, err)
			continue
		}

		ripleyReq, err := conv.ConvertToRipley(linkerdReq, *newHost, *https)
		if err != nil {
			log.Printf("failed to convert request: %v", err)
			continue
		}

		if err := encoder.Encode(ripleyReq); err != nil {
			log.Printf("failed to encode ripley request: %v", err)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading input: %v", err)
	}
}