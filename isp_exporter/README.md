# isp_exporter

A small Go Prometheus exporter that discovers the host's current public IP using `api.ipify.org`, performs reverse DNS, and records both as labels. I use it to detect WAN address/provider changes and correlate connectivity events without relying on a router-specific API.

It runs once at startup and then on a cron schedule. Metrics are served at `/metrics`, defaulting to `0.0.0.0:9353`.

| Flag / environment variable | Default | Meaning |
| --- | --- | --- |
| `--listen` / `LISTEN` | `0.0.0.0` | HTTP listen address |
| `--port` / `PORT` | `9353` | HTTP port |
| `--interval` / `INTERVAL` | `*/20 * * * *` | Five-field cron schedule |
| `--debug` / `DEBUG` | `false` | Add source locations to logs |

```sh
docker run --rm -p 9353:9353 -e INTERVAL='*/10 * * * *' davidcollom/isp_exporter:0.0.0
```

The container requires outbound HTTPS and DNS. The IP address and reverse-DNS hostname become Prometheus labels, so regard the metrics as potentially identifying data.
