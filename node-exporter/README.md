# node-exporter

This is a manifest/wrapper directory for Prometheus node-exporter. The Dockerfile references the upstream image for the pinned version while `PLATFORMS` lists the architectures that the build workflow builds with buildx.

It exists to give my mixed-architecture nodes a consistent `davidcollom/node-exporter:<VERSION>` reference. Runtime flags, mounts, and security requirements are unchanged from upstream.

Typical host monitoring needs read-only access to host `/proc`, `/sys`, and the root filesystem. Follow the upstream deployment guidance and do not grant writable host mounts.
