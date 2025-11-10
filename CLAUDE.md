# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Building and Testing
- **Build**: `go build -o ripley main.go`
- **Test**: `go test pkg/*.go`
- **Run sample server**: `go run etc/dummyweb.go` (starts HTTP server on :8080)

### Running Ripley
- **Basic usage**: `cat etc/requests.jsonl | ./ripley`
- **With pacing**: `./ripley -pace "10s@1 10s@5 1h@10"`
- **Dry run**: `./ripley -pace "30s@1" -dry-run`
- **Silent output**: `./ripley -silent`

### Tools
- **Linkerd converter**: `go run tools/linkerdxripley/main.go`
  - Convert Linkerd access logs to Ripley format
  - Usage: `cat linkerd.jsonl | go run tools/linkerdxripley/main.go -host localhost:8080 > ripley.jsonl`

### Release Process
- Push a new tag to `main` branch to trigger GoReleaser
- GoReleaser configuration in `.goreleaser.yaml`

## Architecture

### Core Components

**Main Package Structure:**
- `main.go`: CLI entry point with command-line flags
- `pkg/replay.go`: Core replay orchestrator that coordinates the entire process
- `pkg/request.go`: HTTP request structures and validation
- `pkg/pace.go`: Rate pacing engine with support for multiple phases
- `pkg/client.go`: HTTP client worker pool and request execution

**Data Flow:**
1. JSONL requests read from STDIN via `bufio.Scanner`
2. `pacer` controls replay timing based on original timestamps and rate multipliers
3. Worker pool of HTTP clients (`numWorkers` goroutines) processes requests concurrently
4. Results streamed to STDOUT as JSONL

**Key Concepts:**
- **Phases**: Time-bounded periods with different rate multipliers (e.g., "10s@1 5m@10")
- **Rate Pacing**: Maintains original traffic patterns while scaling by rate multipliers
- **Request Format**: JSON with required fields: `url`, `method`, `timestamp`
- **Worker Pool**: Configurable number of HTTP client workers for concurrent processing

### Tools Architecture

**Linkerd Converter (`tools/linkerdxripley/`):**
- Converts Linkerd JSONL access logs to Ripley request format
- Supports URL rewriting and HTTP-to-HTTPS upgrade
- Modular converter package with clean separation of concerns

### Configuration

**HTTP Client Settings:**
- Default timeout: 10 seconds
- Default max idle connections per host: 10,000
- Configurable max connections per host (unlimited by default)
- Redirect handling: Uses last response (no follow)

**Performance Tuning:**
- Worker count defaults to `runtime.NumCPU() * 2`
- Connection pooling optimized for high-throughput scenarios
- Memory and CPU profiling support via command-line flags

## Input/Output Format

**Request Format (JSONL):**
```json
{
  "url": "http://localhost:8080/path",
  "method": "GET|POST|PUT|DELETE|etc",
  "timestamp": "2021-11-08T18:59:58.9Z",
  "headers": {"Accept": "text/plain"},
  "body": "{\"key\": \"value\"}"
}
```

**Result Format (JSONL):**
```json
{
  "statusCode": 200,
  "latency": 3915447,
  "request": { ... },
  "error": "optional error message"
}
```

## Development Notes

- Go version requirement: 1.22+
- Strict mode available for development (`-strict` flag causes panic on bad input)
- Comprehensive validation for HTTP methods and required fields
- Built-in statistics reporting with configurable intervals