# Docker container images

This repository is a collection of container images used in my homelab and Kubernetes environments. Some directories are deliberately small wrappers around an upstream image; others rebuild upstream software to add multi-architecture support or extra runtime dependencies; the exporter and automation directories contain applications written specifically for this repository.

The images are published under the `davidcollom` Docker Hub namespace. Each directory is an independent build context and contains a `VERSION` file used as the image tag. Most also contain `PLATFORMS`, which records the intended target architectures.

## Why these images exist

Maintaining these images here gives me one repeatable place to:

- pin the versions deployed in my homelab;
- publish a consistent image name for AMD64, ARM64, and ARMv7 machines;
- add tools or defaults missing from otherwise useful upstream images;
- package small, purpose-built monitoring and Kubernetes utilities;
- rebuild upstream projects into smaller or more suitable runtime images.

These are personal operational images, not guaranteed drop-in replacements for their upstream projects. Review the Dockerfile and directory README before deployment, especially where an image carries opinionated defaults or needs privileged access.

## Image catalogue

| Image | Kind | Purpose / customization |
| --- | --- | --- |
| [alertmanager-github-receiver](alertmanager-github-receiver/) | Upstream rebuild | Builds M-Lab's Alertmanager-to-GitHub receiver as a static, distroless image. |
| [ansible](ansible/) | Customized toolbox | Alpine Ansible runner with SSH, Git, Kubernetes, and commonly used collections/tools. |
| [bind9](bind9/) | Customized service | BIND 9 with container-friendly permissions and foreground startup. |
| [bind9-exporter](bind9-exporter/) | Upstream rebuild | Multi-architecture Prometheus BIND exporter. |
| [cert-exporter](cert-exporter/) | Upstream rebuild | Pinned certificate expiry exporter in a small non-root image. |
| [ddclient](ddclient/) | Thin wrapper | A pinned Alpine `ddclient` command image. |
| [flexget](flexget/) | Customized service | FlexGet with Node, Transmission RPC, Cloudflare helpers, and ARM compatibility workaround. |
| [home-assistant](home-assistant/) | Customized upstream | Home Assistant with `ffmpeg` explicitly available. |
| [hub3-exporter](hub3-exporter/) | Local Go project | Exposes Virgin Media/UPC Hub 3 DOCSIS channel data to Prometheus. |
| [isp_exporter](isp_exporter/) | Local Go project | Tracks the current public IP and reverse-DNS hostname as Prometheus metrics. |
| [kubectl](kubectl/) | Customized CLI | Reuses the matching k3s upgrade image as a version-pinned `kubectl` client. |
| [mercedes-me-exporter](mercedes-me-exporter/) | Local Go project | Publishes Mercedes vehicle data and manages OAuth refresh tokens through a Kubernetes Secret. |
| [minio](minio/) | Manifest composition | References architecture-specific upstream MinIO images for one multi-arch tag. |
| [mkv-auto](mkv-auto/) | Upstream customization | Packages `mkv-auto` with a repository-owned configuration. |
| [node-exporter](node-exporter/) | Manifest composition | Provides one tag across upstream architecture-specific node-exporter images. |
| [node-problem-detector](node-problem-detector/) | Upstream rebuild | Builds Kubernetes node-problem-detector with journald support. |
| [packt-sync](packt-sync/) | Local Perl utility | Synchronizes entitled Packt e-books into a mounted directory. |
| [polipo](polipo/) | Upstream rebuild | Builds the discontinued Polipo proxy from source for multiple architectures. |
| [rsync](rsync/) | Thin wrapper | Minimal Alpine image whose entrypoint is `rsync`. |
| [speedtest_exporter](speedtest_exporter/) | Local Go project | Runs scheduled internet speed tests and exports the results to Prometheus. |
| [ssh-client](ssh-client/) | Thin wrapper | Version-pinned Alpine OpenSSH client. |
| [stubby](stubby/) | Upstream rebuild | Builds the Stubby DNS-over-TLS resolver from source. |
| [stuck-pod-reaper](stuck-pod-reaper/) | Local Go project | Detects, and optionally deletes, old controller-owned Kubernetes pods stuck starting. |
| [system-upgrade-os-patch](system-upgrade-os-patch/) | Local Go project | Returns a calendar-derived version redirect for system-upgrade orchestration. |
| [tailscale](tailscale/) | Customized upstream | Adds Kubernetes state storage, userspace networking, routes, and optional DNAT startup logic. |
| [three-drop-sync](three-drop-sync/) | Local Go project | Experimental NAS archive of 3Drop likes/collections with concurrent model downloads; account integration is pending verification. |
| [transmission](transmission/) | Customized service | Transmission daemon with persistent configuration, environment overrides, UID/GID mapping, and blocklists. |
| [unifi-cert-updater](unifi-cert-updater/) | Local Go project | Copies a Kubernetes TLS Secret into UniFi OS, activates it, and prunes old certificates. |
| [unifi-exporter](unifi-exporter/) | Upstream rebuild | Builds an upstream UniFi Prometheus exporter for multiple architectures. |
| [version-checker](version-checker/) | Upstream rebuild | Packages Jetstack version-checker as a non-root Alpine image. |
| [vmhub5_exporter](vmhub5_exporter/) | Local Go project | Exposes Virgin Media Hub 5 provisioning, PON, and optical data to Prometheus. |

## Building and publishing

The root build script builds and pushes versioned images, then creates a Docker manifest when a directory contains multiple architecture sources.

```sh
# Build every directory containing VERSION and publish to davidcollom/*
make build

# Build only selected images under a chosen registry namespace
./build.sh my-registry-user speedtest_exporter unifi-cert-updater
```

Set `DOCKER_USER` and `DOCKER_PASSWORD` when using `make login`. The legacy build script expects Docker CLI experimental manifest support to be enabled. It interprets Dockerfile suffixes and `image.<arch>` filenames as architecture names. Modern Docker Buildx may be preferable for new work, but the current files document the repository's existing release workflow.

## Directory convention

- `Dockerfile` defines the build or wraps an upstream image.
- `VERSION` supplies the version/tag passed as the `VERSION` build argument.
- `PLATFORMS` lists intended build targets.
- `image.<arch>` points at an already-published architecture-specific image to include in a manifest.
- `.skip`, when present, prevents the root build script from publishing that directory.

Build a single context locally with:

```sh
docker build --build-arg VERSION="$(cat speedtest_exporter/VERSION)" \
  -t davidcollom/speedtest_exporter:local speedtest_exporter
```

## Security and maintenance notes

Several utilities need credentials or Kubernetes RBAC. Pass credentials through secrets rather than committing them. Grant service accounts only the verbs and namespaces described in each image README. Some images intentionally track old or discontinued upstream software for compatibility; treat them as homelab components and review their exposure before making them internet-facing.

Versions are intentionally explicit. Updating a `VERSION` file does not prove compatibility: rebuild the image, inspect upstream release notes, and exercise the relevant service or metrics endpoint before publishing the tag.
