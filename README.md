# ripley - replay HTTP

Ripley replays HTTP traffic at multiples of the original rate. It simulates traffic ramp up or down by specifying rate phases for each run. For example, you can replay HTTP requests at twice the original rate for ten minutes, then three times the original rate for five minutes, then ten times the original rate for an hour and so on. Ripley's original use case is load testing by replaying HTTP access logs from production applications.

## Install

### Pre-built

#### MasOS
```bash
brew install loveholidays/tap/ripley
```
#### Docker
```bash
docker pull loveholidays/ripley
```
#### Linux

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
seq 10 | xargs -i cat etc/requests.jsonl | ./ripley -pace "10s@1 10s@5 1h@10"
```

## Replaying HTTP traffic

Ripley reads a representation of HTTP requests in [JSON Lines format](https://jsonlines.org/) from `STDIN` and replays them at different rates in phases as specified by the `-pace` flag.

An example ripley request:

```JSON
{
    "url": "http://localhost:8080/",
    "verb": "GET",
    "timestamp": "2021-11-08T18:59:59.9Z",
    "headers": {"Accept": "text/plain"}
}
```

`url`, `verb` and `timestamp` are required, `headers` are optional.

`-pace` specifies rate phases in `[duration]@[rate]` format. For example, `10s@5 5m@10 1h30m@100` means replay traffic at 5x for 10 seconds, 10x for 5 minutes and 100x for one and a half hours. The run will stop either when ripley stops receiving requests from `STDIN` or when the last phase elapses, whichever happens first.

Ripley writes request results as JSON Lines to `STDOUT`

```bash
echo '{"url": "http://localhost:8080/", "verb": "GET", "timestamp": "2021-11-08T18:59:50.9Z"}' | ./ripley | jq
```

produces

```JSON
{
  "statusCode": 200,
  "latency": 3915447,
  "request": {
    "verb": "GET",
    "url": "http://localhost:8080/",
    "body": "",
    "timestamp": "2021-11-08T18:59:50.9Z",
    "headers": null
  }
}
```

Results output can be suppressed using the `-silent` flag.

It is possible to collect and print a run's statistics:

```bash
seq 10 | xargs -i cat etc/requests.jsonl | ./ripley -pace "10s@1 10s@5 1h@10" -silent -stats | jq
```

```JSON
{
  "totalRequests": 100,
  "statusCodes": {
    "200": 100
  },
  "latencyMicroseconds": {
    "max": 2960,
    "mean": 2008.25,
    "median": 2085.5,
    "min": 815,
    "p95": 2577,
    "p99": 2876,
    "stdDev": 449.1945986986041
  }
}
```

It is possible to disable sending HTTP requests to the targets with the `-dry-run` flag:

```bash
cat etc/requests.jsonl | ./ripley -pace "30s@1" -dry-run
```

## Running the tests

```bash
go test pkg/*go
```
