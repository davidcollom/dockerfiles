# unifi-exporter

This image rebuilds the upstream `adriansaul/unifi_exporter` Go project in Alpine and publishes it for the architectures used by my monitoring stack. It exists to pin the exporter release behind a consistent image namespace and avoid depending on a single upstream architecture.

The exporter runs as `nobody` and exposes port `9130`.

```sh
docker run --rm -p 9130:9130 davidcollom/unifi-exporter:0.4.0 --help
```

Pass controller URL and credentials using the upstream configuration mechanism. Prefer a read-only UniFi monitoring account and keep the exporter on a trusted network.
