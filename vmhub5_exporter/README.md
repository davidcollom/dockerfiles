# vmhub5_exporter

A Go Prometheus exporter written for the Virgin Media Hub 5. It polls the router's local REST endpoints and exposes WAN provisioning lease data, DS-Lite state, PON uptime/access/firewall state, data and VoIP status, plus optical transceiver measurements. This makes fibre/router health observable without scraping the web interface.

The exporter serves `/metrics` on port `8080`, polls immediately, and refreshes every 10 seconds by default.

```sh
docker run --rm -p 8080:8080 davidcollom/vmhub5_exporter:0.0.0.5 \
  --host 192.168.0.1 --interval 30s
```

Flags are `--host`/`-H` and `--interval`/`-i`. The container requires network access to these router endpoints: `/rest/v1/system/gateway/provisioning`, `/rest/v1/pon/state`, and `/rest/v1/pon/status`. It stores no state. A Grafana dashboard export is included as `dashboard.json`.
