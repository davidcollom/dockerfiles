# Raspberry Pi NFD discovery

Minimal Go-based Raspberry Pi hardware discovery for the Node Feature Discovery (NFD) `local` source.

Reads the host's device-tree model and optional `/proc/cpuinfo` revision, and atomically writes an NFD feature file to `features.d/rpi`. Does **not** require Kubernetes API permissions. Does **not** update health/status labels on every sample.

## Labels

- `feature.node.kubernetes.io/rpi-present=true`
- `feature.node.kubernetes.io/rpi-model=3-bplus` (model-dependent)
- `feature.node.kubernetes.io/rpi-revision=a020d3` (if exposed by the kernel)

## Requirements

Your deployed NFD worker must have the `local` source enabled **and must mount the same host path** (`/etc/kubernetes/node-feature-discovery/features.d`) as this DaemonSet.

Example local host test:

```sh
go test ./...
go run . -model /proc/device-tree/model -cpuinfo /proc/cpuinfo \
  -output /tmp/rpi-nfd-features -interval 5m
```

Build and publish the multi-arch image before enabling the draft homelab deployment:

```sh
docker buildx build --platform linux/arm/v7,linux/arm64,linux/amd64 \
  -t davidcollom/rpi-nfd-discovery:0.1.0 --push .
```

Image is not published merely by committing these source files. The image is intentionally distroless/non-root. The DaemonSet needs a writable host-mounted NFD features directory.

Throttling and undervoltage are deliberately out of scope for static NFD labels; they belong in Prometheus telemetry and alerts. A later exporter can expose `vcgencmd get_throttled` and associated events.
