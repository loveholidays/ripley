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

## Long-lived responses and SSE

Ripley consumes response bodies incrementally without retaining their contents.
SSE sessions are ordinary HTTP requests: supply the captured headers and an optional
`disconnectAfter` (a Go duration string) to reproduce the client's session lifetime.

```json
{"url":"http://localhost:8080/sse","method":"GET","timestamp":"2021-11-08T18:59:58.9Z","headers":{"Accept":"text/event-stream","Last-Event-ID":"42"},"disconnectAfter":"5m"}
```

A positive `disconnectAfter` is measured in log time from the request's
`timestamp`, so a session keeps its place in the replayed traffic: at `@10`, a
`"30s"` session lasts 3 seconds, and one that spans a phase change follows both
rates. It covers any wait for `-workers`, connection setup and response-body
consumption, and replaces `-timeout` for that request only. A session whose
time runs out while it waits for `-workers` is never sent, and is reported as an
error. Zero or omitted `disconnectAfter` retains the existing client
timeout; negative durations are invalid.

When the session deadline expires first, the result has `"disconnected": true`
and an empty `error`. This includes a server that never sends response headers:
there is no separate header timeout, so such a session reports
`"disconnected": true` with `"statusCode": 0`. If the server ends the response
first, the result is a normal completion. Other failures remain errors, with
`statusCode` zero as before. `latency` includes setup and response-body
consumption. Ordinary results omit the new `disconnected` field. The echoed
request normalizes durations, so `"5m"` appears as `"5m0s"`.

Disconnected requests count towards `ripley_requests_total`, but not towards
`ripley_request_duration_seconds` or `ripley_response_status_total`: their
length was set by the input rather than measured, and they may have no status.

Silence is legal in SSE and does not terminate a session. Each request runs on
its own goroutine, so long-lived responses never hold back the requests after
them. `-workers` caps the requests in flight, and unlimited is the default;
at a cap, the replay waits and falls behind its schedule.

Ripley does not parse individual SSE events or automatically reconnect. Captured
reconnections must appear as separate requests in the input. SSE does not require
HTTP/1.1; the ordinary transport can negotiate HTTP/2 over HTTPS.

Try the sample server in one terminal with `go run etc/dummyweb.go`. Its `/sse`
streams until the client disconnects; `/sse?lifetime=10s` makes the server end
the stream itself after 10 seconds. Then:

```bash
go build -o ripley main.go
printf '%s\n' '{"url":"http://localhost:8080/sse","method":"GET","timestamp":"2021-11-08T18:59:58.9Z","headers":{"Accept":"text/event-stream"},"disconnectAfter":"5s"}' | ./ripley
```

## Slow clients

An optional `network` field limits how fast Ripley reads the response body, to
replay clients on slow connections. It is either a Chrome DevTools preset:

```json
{"url":"http://localhost:8080/sse","method":"GET","timestamp":"2021-11-08T18:59:58.9Z","disconnectAfter":"5m","network":"3g"}
```

| `network` | Download speed |
| --- | --- |
| `"3g"` | 400 kbps (50 KB/s) |
| `"slow-4g"` | 1440 kbps (180 KB/s) |
| `"fast-4g"` | 8100 kbps (1 MB/s) |

or, for any other speed, a whole number of kilobits per second, as in Chrome's
custom profiles: `"network": 750`. Omitted or `0` means unlimited. Unknown
presets, fractions, negative speeds and speeds above 10 Gbps are rejected.
Results echo the speed as a number, so `"3g"` appears as `400`.

Only the response body is limited: request bodies, headers and connection
setup are not, and no latency is added. Each replayed request gets its own
speed, as if it came from a different device; Chrome instead shares one speed
between a page's requests.

Ripley reads in chunks of at most a quarter of a second's worth (and at most
8 KiB) and sleeps between them, so the limit is an average with small bursts.
When the server sends more slowly than the limit, Ripley reads data as it
arrives, and quiet periods earn at most a quarter second's allowance: a burst
after a silent spell, as is common with SSE, is still read at the limited
speed, as in Chrome.

The server sees the slow client through TCP backpressure, but kernel and HTTP/2
buffering let it send somewhat ahead of what Ripley has read. Go transparently
decompresses gzip responses unless the request sets `Accept-Encoding`, so by
default the limit applies to the decompressed bytes; set `Accept-Encoding` to
limit the bytes as sent. A throttled request can overrun `-timeout` by up to one
chunk's sleep, because the timeout is noticed on the next read;
`disconnectAfter` interrupts the sleep immediately.

## Metrics and profiling

Enable Prometheus metrics with `-metricsServerEnable`. Ripley listens on
`0.0.0.0:8081` by default and exposes metrics at `/metrics`.

Use `-pushgateway-url`, `-pushgateway-job`, and `-pushgateway-interval` to
persist metrics for short-lived Jobs. Ripley pushes immediately, at the configured
interval, and when it exits normally.

Use `-pprof-enable` with `-metricsServerEnable` to expose Go profiles on the same
listener. Keep this endpoint private. During an active run, use port-forwarding to
inspect:

```text
/debug/pprof/goroutine?debug=1
/debug/pprof/heap
/debug/pprof/profile?seconds=30
/debug/pprof/threadcreate
/debug/pprof/block
/debug/pprof/mutex
/debug/pprof/trace?seconds=5
```

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
A successful merge to `main` creates a release tag and starts GoReleaser.

- Normal merge: increments the minor version, for example `v0.1.3` to `v0.2.0`.
- Merge commit message containing `#major`: increments the major version, for example `v0.1.3` to `v1.0.0`.

Do not create release tags manually.
