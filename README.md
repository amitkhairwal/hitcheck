# hitcheck

A small Go command line tool that does two things: checks whether a list of HTTP endpoints is up, and fires a burst of requests at one URL to see how it holds up.

I wrote it because I kept typing throwaway `curl` loops while testing services, and I wanted one command that printed a clean table and could also act as a quick load generator. It builds to a single binary. The only dependency is `yaml.v3`, for the config file.

## Build

```
go build -o hitcheck .
```

## Commands

`check`, `watch` and `load`. Run `hitcheck <command> -h` for the flags.

### check

Run the checks once. Pass URLs directly or point it at a YAML file.

```
hitcheck check https://example.com https://api.github.com
hitcheck check -config config.yaml
```

Output from a run against a local server (one endpoint expects a 404, one port has nothing listening):

```
ENDPOINT    STATUS  CODE  LATENCY  NOTE
home        UP      200   32.3ms   ok
missing     UP      404   32.7ms   ok
wrong-port  DOWN    -     3.7ms    connection refused

2/3 healthy
```

Endpoints are checked in parallel, so the run takes about as long as the slowest one. The exit code is `1` if anything is down, so it works in cron or a CI step. Add `-json file` or `-csv file` to save the results. Flags go before bare URLs, which is how Go's `flag` package works.

### watch

Same checks, repeated on an interval, with the table redrawn in place.

```
hitcheck watch -config config.yaml -interval 5s
```

### load

Send a fixed number of requests to one URL through a pool of workers, then print latency percentiles.

```
hitcheck load -url http://localhost:8099/ -n 500 -c 10
```

```
running 500 requests at concurrency 10...

Target       http://localhost:8099/
Requests     500 (10 concurrent)
Duration     1.022s
Throughput   489 req/s
Success      500 (100.0%)
Failed       0

Latency
  min   1.8ms
  mean  9.5ms
  p50   2.9ms
  p90   4.7ms
  p95   6.4ms
  p99   18.2ms
  max   1020.9ms

Status codes
  200    500
```

| flag | meaning | default |
| --- | --- | --- |
| `-url` | target URL (required) | |
| `-n` | total requests | 200 |
| `-c` | concurrent workers, 1000 at most | 20 |
| `-method` | HTTP method | GET |
| `-timeout` | per-request timeout | 10s |
| `-H` | header as `Key: Value`, repeatable | |
| `-json`, `-csv` | write the summary to a file | |

```
hitcheck load -url https://api.example.com/ping -H "Authorization: Bearer $TOKEN" -n 500 -c 25
```

## Config

Copy `config.example.yaml` to `config.yaml` and list your endpoints. Durations are Go strings like `5s` or `500ms`. `expect_status` lets a non-200 count as healthy, for endpoints that should return 404 or 401.

## How it works

- `check` starts one goroutine per endpoint and waits on a `sync.WaitGroup`.
- `load` uses a fixed worker pool. One goroutine feeds jobs into a channel, the workers read from it, and a single collector reads results from a second channel. The pool size is the cap on in-flight requests.
- Percentiles use the nearest-rank method on the sorted samples.
- The JSON export keeps raw nanoseconds (`*_ns` fields). The CSV export is in milliseconds.

## Tests

```
go test ./...
```

Config parsing, the stats maths and the load runner have tests. The `check` command and the report formatting do not yet.
