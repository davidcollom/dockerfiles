# speedtest_exporter

A Go Prometheus exporter written to run repeatable internet speed tests from my network. It measures download throughput, upload throughput, transferred-byte estimates, and latency, making WAN performance visible alongside other infrastructure metrics.

It runs immediately, then according to a cron schedule, and exposes `/metrics` on port `9353` by default.

| Flag / environment variable | Default | Meaning |
| --- | --- | --- |
| `--listen` / `LISTEN` | `0.0.0.0` | HTTP listen address |
| `--port` / `PORT` | `9353` | HTTP port |
| `--interval` / `INTERVAL` | `*/20 * * * *` | Five-field cron schedule |
| `--server` / `SERVER` | empty | Optional speed-test server ID |
| `--debug` / `DEBUG` | `false` | Add source locations to logs |

```sh
docker run --rm -p 9353:9353 -e INTERVAL='0 * * * *' \
  davidcollom/speedtest_exporter:0.1.0
```

Tests consume meaningful bandwidth. Choose an interval appropriate to your connection and data allowance. Only AMD64 and ARM64 are currently declared in `PLATFORMS`.
