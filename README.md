# hitcheck

A small command line tool for checking whether a bunch of HTTP endpoints are
up, and for throwing load at one of them to see how it holds up. Written in Go,
no runtime dependencies, single binary.

I built it because I kept writing throwaway `curl` loops to sanity check
services during development and wanted something that printed a clean table and
could also act as a quick load generator.

## Install

```
go build -o hitcheck .
```

Or run it without building:

```
go run . check https://example.com
```

## Usage

There are three subcommands: `check`, `watch` and `load`.

### check

Run a one-off health check. Pass URLs directly:

```
hitcheck check https://api.github.com https://httpbin.org/get
```

or load a list of endpoints from YAML:

```
hitcheck check -config config.yaml
```

Output:

```
ENDPOINT          STATUS  CODE  LATENCY   NOTE
github            UP      200   142.3ms   ok
httpbin-get       UP      200   311.0ms   ok
not-found         UP      404   208.7ms   ok
broken            DOWN    -     5001.2ms  timeout

3/4 healthy
```

The process exits with code `1` if any endpoint is down, so it drops straight
into a cron job or a CI step.

Export the results with `-json` or `-csv`:

```
hitcheck check -config config.yaml -csv report.csv
```

Flags have to come before any bare URLs, e.g.
`hitcheck check -csv report.csv https://example.com` (this is how Go's flag
parser works).

### watch

Same checks as above, but re-run on an interval and redraw the table in place.
Good for leaving open on a second monitor.

```
hitcheck watch -config config.yaml -interval 5s
```

Ctrl-C to stop.

### load

Fire a fixed number of requests at a single URL through a pool of workers and
report latency percentiles.

```
hitcheck load -url https://httpbin.org/get -n 1000 -c 50
```

```
Target       https://httpbin.org/get
Requests     1000 (50 concurrent)
Duration     6.214s
Throughput   161 req/s
Success      1000 (100.0%)
Failed       0

Latency
  min    98.0ms
  mean   305.1ms
  p50    291.4ms
  p90    402.7ms
  p95    448.9ms
  p99    693.2ms
  max    1004.6ms

Status codes
  200    1000
```

Flags:

| flag        | meaning                              | default |
|-------------|--------------------------------------|---------|
| `-url`      | target URL (required)                |         |
| `-n`        | total requests to send               | 200     |
| `-c`        | concurrent workers (max 1000)        | 20      |
| `-method`   | HTTP method                          | GET     |
| `-timeout`  | per-request timeout                  | 10s     |
| `-H`        | request header, repeatable           |         |
| `-json`     | write the summary to a JSON file     |         |
| `-csv`      | write the summary to a CSV file      |         |

Custom headers:

```
hitcheck load -url https://api.example.com/v1/ping \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: application/json" \
  -n 500 -c 25
```

## Config file

See `config.example.yaml`. Durations are written as Go duration strings
(`5s`, `500ms`, `2m`). `expect_status` lets you treat a non-200 response as
healthy, which is useful for endpoints that are supposed to return 404 or 401.

## How it works

- **check** runs one goroutine per endpoint and waits on a `sync.WaitGroup`,
  so total time is roughly the slowest endpoint rather than the sum.
- **load** uses a fixed worker pool. A feeder goroutine pushes jobs onto a
  channel, the workers pull from it, and results come back on a second channel
  that a single collector drains. The pool size caps concurrency so the target
  is not hit with more in-flight requests than asked for.
- Percentiles use the nearest-rank method over the sorted sample set.

## Notes

Latencies in the terminal are shown in milliseconds. The JSON export keeps raw
nanosecond counts (the `*_ns` fields) so nothing is lost to rounding; the CSV
export converts to milliseconds for spreadsheet friendliness.
