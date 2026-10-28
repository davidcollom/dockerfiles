# OpenCode

The official OpenCode v2 image with Git, SSH client, Bash, curl, jq, ripgrep,
Python, Node.js, npm, GitHub CLI, Docker CLI and Buildx for a persistent coding workspace. Runs as UID/GID 1000.
The upstream OpenCode tag is pinned in `UPSTREAM_VERSION` in the Dockerfile;
`VERSION` identifies this derived image. `PLATFORMS` follows the shared build workflow.
Docker tooling is client-only: no Docker daemon, host socket, or cluster credentials
are included. A separately isolated builder is required for container builds.

## Manual build while GitHub Actions is unavailable

From the Dockerfiles repository root:

```sh
version=$(cat opencode/VERSION)
platforms=$(paste -sd, opencode/PLATFORMS)
docker buildx build --builder local --platform "$platforms" \
  --build-arg VERSION="$version" \
  -t "ghcr.io/davidcollom/opencode:$version" \
  -t "davidcollom/opencode:$version" \
  --push opencode/
```

Use the existing Docker credential helper; do not put registry tokens in this folder.
Set `OPENCODE_PASSWORD` using a secret. The OpenCode v2 HTTP username is `opencode`.
Persist `/home/opencode` on block storage (SQLite sessions/authentication), and mount
NFS project storage at `/workspace`. Configure providers through `/etc/opencode/opencode.json`.
The Kubernetes resources are in the homelab repository under `clusters/home/opencode`.
