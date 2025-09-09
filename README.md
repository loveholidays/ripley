# ripley - replay HTTP

Ripley replays HTTP traffic at multiples of the original rate. While similar tools usually generate load at a set rate, such as 100 requests per second, ripley uses request timestamps, for example those recorded in access logs, to more accurately represent real world load. It simulates traffic ramp up or down by specifying rate phases for each run. For example, you can replay HTTP requests at twice the original rate for ten minutes, then three times the original rate for five minutes, then ten times the original rate for an hour and so on. Ripley's original use case is load testing by replaying HTTP access logs from production applications.

## Install

```bash
# go >= 1.17
# Using `go get` to install binaries is deprecated.
# The version suffix is mandatory.
go install github.com/loveholidays/ripley@latest

# go < 1.17
go get github.com/loveholidays/ripley
```

### Homebrew

```bash
brew install loveholidays/tap/ripley
```

### Docker
```bash
docker pull loveholidays/ripley
```

### Linux
Grab the latest OS/Arch compatible binary from our [Releases](https://github.com/loveholidays/ripley/releases) page.

### From source
```bash
git clone git@github.com:loveholidays/ripley.git
cd ripley
go build -o ripley main.go
```

#### Quickstart from source
Run a web server to replay traffic against

```bash
go run etc/dummyweb.go
```

Loop 10 times over a set of HTTP requests at 1x rate for 10 seconds, then at 5x for 10 seconds, then at 10x for the remaining requests

```bash
seq 10 | xargs -I{} cat etc/requests.jsonl | ./ripley -pace "10s@1 10s@5 1h@10"
```

## Replaying HTTP traffic

Ripley reads a representation of HTTP requests in [JSON Lines format](https://jsonlines.org/) from `STDIN` and replays them at different rates in phases as specified by the `-pace` flag.

An example ripley request:

```JSON
{
  "url": "http://localhost:8080/",
  "method": "POST",
  "body": "{\"foo\": \"bar\"}",
  "headers": {
    "Accept": "text/plain"
  },
  "timestamp": "2021-11-08T18:59:58.9Z"
}
```

`url`, `method` and `timestamp` are required, `headers` and `body` are optional.

`-pace` specifies rate phases in `[duration]@[rate]` format. For example, `10s@5 5m@10 1h30m@100` means replay traffic at 5x for 10 seconds, 10x for 5 minutes and 100x for one and a half hours. The run will stop either when ripley stops receiving requests from `STDIN` or when the last phase elapses, whichever happens first.

Ripley writes request results as JSON Lines to `STDOUT`

```bash
echo '{"url": "http://localhost:8080/", "method": "GET", "timestamp": "2021-11-08T18:59:50.9Z"}' | ./ripley | jq
```

produces

```JSON
{
  "statusCode": 200,
  "latency": 3915447,
  "request": {
    "method": "GET",
    "url": "http://localhost:8080/",
    "body": "",
    "timestamp": "2021-11-08T18:59:50.9Z",
    "headers": null
  }
}
```

Results output can be suppressed using the `-silent` flag.

For an example of working with ripley's output to generate statistics, refer to https://gist.github.com/georgemalamidis-lh/39b4f4a6c9c82f6cc8b7370219e93cd2

```bash
cat etc/requests.jsonl | ./ripley | go run ripley_stats.go | jq
```

```JSON
{
  "totalRequests": 10,
  "statusCodes": {
    "200": 10
  },
  "latency": {
    "max": 2074819,
    "mean": 968998.6,
    "median": 843486,
    "min": 696708,
    "p95": 1548438.5,
    "p99": 1548438.5,
    "stdDev": 377913.54080112034
  }
}
```

It is possible to disable sending HTTP requests to the targets with the `-dry-run` flag:

```bash
cat etc/requests.jsonl | ./ripley -pace "30s@1" -dry-run
```

## Converting Linkerd Access Logs

The `linkerdxripley` tool converts [Linkerd](https://linkerd.io/) JSONL access logs into Ripley's request format, enabling you to replay production Linkerd traffic for load testing.

### Installation

Build from source:
```bash
cd tools/linkerdxripley
go build -o linkerdxripley main.go
```

Or install directly:
```bash
go install github.com/loveholidays/ripley/tools/linkerdxripley@latest
```

### Usage

Basic conversion:
```bash
cat linkerd.jsonl | linkerdxripley > ripley.jsonl
```

Convert with host replacement:
```bash
cat linkerd.jsonl | linkerdxripley -host localhost:8080 > ripley.jsonl
```

Convert with HTTPS upgrade:
```bash
cat linkerd.jsonl | linkerdxripley -host localhost:8443 -https > ripley.jsonl
```

Full pipeline (convert and replay):
```bash
cat linkerd.jsonl | linkerdxripley -host localhost:8080 | ./ripley -pace "1m@2 5m@5"
```

### Input Format

The tool expects Linkerd JSONL access logs with these fields:

```json
{
  "client.addr": "192.168.1.100:12345",
  "client.id": "service.namespace.serviceaccount.identity.linkerd.cluster.local",
  "host": "api.example.com",
  "method": "GET",
  "processing_ns": "50000",
  "request_bytes": "256",
  "status": 200,
  "timestamp": "2025-09-03T15:30:32.928995068Z",
  "total_ns": "2500000",
  "trace_id": "abc123",
  "uri": "http://api.example.com/api/v1/data?id=123",
  "user_agent": "MyApp/1.0",
  "version": "HTTP/2.0"
}
```

Required fields: `method`, `timestamp`, `uri`

### Output Format

Produces Ripley-compatible JSONL:

```json
{
  "method": "GET",
  "url": "http://localhost:8080/api/v1/data?id=123",
  "body": "",
  "timestamp": "2025-09-03T15:30:32.928995068Z",
  "headers": {
    "User-Agent": "MyApp/1.0"
  }
}
```

### Command Options

- `-host <host:port>` - Replace the original host with a new target host
- `-https` - Upgrade HTTP requests to HTTPS (useful for local testing with TLS)
- `-help` - Show usage information

## Running the tests

```bash
go test pkg/*.go
```

## Releasing new versions
Push a new tag to `main` to trigger the GoReleaser process.
