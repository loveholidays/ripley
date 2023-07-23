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

For an example of working with ripley's output to generate statistics, refer to [etc/stats.go](https://github.com/loveholidays/ripley/blob/main/etc/stats.go)

```bash
cat etc/requests.jsonl | ./ripley | go run ./etc/stats.go | jq
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

`-strict` mode causes ripley to panic when encountering an error:

```bash
cat etc/requests.jsonl | ./ripley -strict
```

```JSON
{
  "statusCode": 0,
  "latency": 0,
  "request": {
    "method": "PROPFIND",
    "url": "http://localhost:8080/",
    "body": "",
    "timestamp": "2021-11-08T19:00:00.03Z",
    "headers": null
  },
  "error": "invalid method: PROPFIND"
}
```

```bash
panic: invalid method: PROPFIND
```

If `-strict` isn't specified, ripley will print the problematic request, skip it and exit with a non zero exit code when it's done processing all requeests. Problematic requests aren't replayed.

```bash
cat etc/requests.jsonl | ./ripley -dry-run > /dev/null
```
```bash
echo $? #=> 126
```

## ripleysort

If the access logs that ripley replays are not strictly ordered by their timestamps, such as when combining access logs from several different sources, the `ripleysort` command can stream sort them, without loading all of the requests in memory. This is useful when sorting large datasets that won't fit in the host system's available memory. The `-bufferlen` flag can be used to configure `ripleysort`'s request buffer in to order to correctly sort certain datasets. In `-strict` mode, `ripleysort` will panic when it emits an out of order request. Knowing when `ripleysort` emits an out of order request is useful when tuning `-bufferlen`.

```bash
cat etc/outoforderrequests.jsonl | jq '.["timestamp"]'
```
```bash
"2021-11-08T18:59:50.9Z"
"2021-11-08T18:59:52.9Z"
"2021-11-08T18:59:51.9Z"
"2021-11-08T18:59:53.9Z"
"2021-11-08T18:59:54.9Z"
"2021-11-08T18:59:55.9Z"
"2021-11-08T18:59:56.9Z"
"2021-11-08T18:59:58.9Z"
"2021-11-08T18:59:57.9Z"
"2021-11-08T18:59:59.9Z"
"2021-11-08T19:00:00.01Z"
"2021-11-08T19:00:00.02Z"
"2021-11-08T19:00:00.03Z"
"2021-11-08T19:00:00.04Z"
"2021-11-08T19:00:00.00Z"
```
```bash
cat etc/outoforderrequests.jsonl | ./ripleysort | jq '.["timestamp"]'
```
```bash
"2021-11-08T18:59:50.9Z"
"2021-11-08T18:59:51.9Z"
"2021-11-08T18:59:52.9Z"
"2021-11-08T18:59:53.9Z"
"2021-11-08T18:59:54.9Z"
"2021-11-08T18:59:55.9Z"
"2021-11-08T18:59:56.9Z"
"2021-11-08T18:59:57.9Z"
"2021-11-08T18:59:58.9Z"
"2021-11-08T18:59:59.9Z"
"2021-11-08T19:00:00Z"
"2021-11-08T19:00:00.01Z"
"2021-11-08T19:00:00.02Z"
"2021-11-08T19:00:00.04Z"
```

Build `ripleysort` with:
```bash
go build -o ripley main.go
```


## Running the tests

```bash
go test pkg/*.go
```
