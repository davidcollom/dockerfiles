# bind9-exporter

This image compiles `prometheus-community/bind_exporter` at the version stored in `VERSION`, then places the binary in a small Alpine runtime. It exists to provide a consistent multi-architecture image for monitoring the customized BIND deployment.

The exporter listens on port `9119` and runs as `nobody`.

```sh
docker run --rm -p 9119:9119 davidcollom/bind9-exporter:0.8.0 --help
```

Configure BIND's statistics channel and pass the matching exporter flags described by the upstream project. Do not expose the statistics endpoint publicly.
