# mercedes-me-exporter

A Go Prometheus exporter written to make selected Mercedes-Benz vehicle data observable in my Kubernetes monitoring stack. It periodically exports odometer, fuel level, and remaining liquid-fuel range. A second scheduled job refreshes Mercedes OAuth credentials and writes the rotated refresh token back to a Kubernetes Secret. The multi-stage build produces a static binary and a small, non-root Alpine runtime.

Metrics are exposed on port `9353` by default. The application loads a local kubeconfig during development and falls back to in-cluster authentication.

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `EXPORTER_LISTEN` | `0.0.0.0` | Metrics listen address |
| `EXPORTER_PORT` | `9353` | Metrics port |
| `EXPORTER_INTERVAL` | `*/20 * * * *` | Vehicle polling cron schedule |
| `EXPORTER_VIN` | empty | Vehicle identification number |
| `EXPORTER_SECRET` | `mercedesme` | Kubernetes Secret name |
| `NAMESPACE` | `monitoring` | Secret namespace |
| `EXPORTER_DEBUG` | `false` | Debug logging switch |

The Secret must contain base64-backed keys named `TOKEN`, `CLIENT_ID`, and `CLIENT_SECRET`. Its service account needs `get` and `patch` access to that one Secret.

```sh
docker run --rm -p 9353:9353 \
  -e EXPORTER_VIN=YOUR_VIN -e EXPORTER_SECRET=mercedesme -e NAMESPACE=monitoring \
  davidcollom/mercedes-me-exporter:0.1.0
```

This project calls Mercedes APIs and logs API activity. Treat VINs and OAuth material as secrets, review logging before production use, and prefer running it inside the cluster with tightly scoped RBAC.
