# linkerdxripley

A CLI tool to convert Linkerd JSONL format to Ripley format for HTTP traffic replay.

## Overview

This tool reads Linkerd service mesh access logs in JSONL format from stdin and converts them to Ripley's expected format, allowing you to replay real production traffic patterns for load testing.

## Usage

### Basic conversion
```bash
cat linkerd_logs.jsonl | linkerdxripley > ripley_requests.jsonl
```

### With host modification
```bash
cat linkerd_logs.jsonl | linkerdxripley -host localhost:8080 > ripley_requests.jsonl
```

### Full pipeline with Ripley
```bash
cat linkerd_logs.jsonl | linkerdxripley -host staging.api.com:9000 | ripley -pace "10s@1 30s@5"
```

## Options

- `-host string`: Replace the original host in URLs with a new host (optional)
- `-help`: Show usage information

## Input Format (Linkerd JSONL)

```json
{
  "client.addr": "192.168.1.100:12345",
  "client.id": "test-service.default.serviceaccount.identity.linkerd.cluster.local",
  "host": "api-service.test.svc.cluster.local",
  "method": "GET",
  "processing_ns": "50000",
  "request_bytes": "0",
  "status": 200,
  "timestamp": "2025-09-03T15:30:32.928995068Z",
  "total_ns": "2500000",
  "trace_id": "abc123",
  "uri": "http://api-service.test.svc.cluster.local/api/v1/data?id=12345",
  "user_agent": "TestClient/1.0",
  "version": "HTTP/2.0"
}
```

## Output Format (Ripley JSONL)

```json
{
  "method": "GET",
  "url": "http://localhost:8080/api/v1/data?id=12345",
  "timestamp": "2025-09-03T15:30:32.928995068Z",
  "headers": {
    "User-Agent": "TestClient/1.0",
    "Host": "api-service.test.svc.cluster.local"
  }
}
```

## Field Mapping

| Linkerd Field | Ripley Field | Notes |
|---------------|--------------|-------|
| `method` | `method` | HTTP method (GET, POST, etc.) |
| `uri` | `url` | Request URL, optionally with modified host |
| `timestamp` | `timestamp` | RFC3339Nano timestamp |
| `user_agent` | `headers["User-Agent"]` | If present |
| `host` | `headers["Host"]` | Original host preserved in headers |

## Building

```bash
go build -o linkerdxripley main.go
```

## Testing

```bash
# Run unit tests
go test ./pkg/converter/

# Run integration tests
go test .

# Test with sample data
cat testdata/linkerd_sample.jsonl | go run main.go -host localhost:8080
```

## Examples

### Convert production logs for local testing
```bash
kubectl logs -n linkerd-viz deployment/tap --since=1h | \
  grep -E '^{' | \
  linkerdxripley -host localhost:3000 | \
  ripley -pace "1m@0.1"
```

### Replay at different rates against staging
```bash
cat production_linkerd.jsonl | \
  linkerdxripley -host staging.myapi.com | \
  ripley -pace "30s@1 5m@2 10m@5"
```