# alertmanager-github-receiver

This image rebuilds the upstream M-Lab Alertmanager GitHub receiver and copies the static Go binary into a distroless runtime. I keep the build here to pin the receiver version, produce the architectures used in my cluster, and avoid carrying a compiler or package manager at runtime.

The receiver turns matching Prometheus Alertmanager notifications into GitHub issues. Configuration and command-line behavior come from the [upstream project](https://github.com/m-lab/alertmanager-github-receiver).

```sh
docker build --build-arg VERSION="$(cat VERSION)" -t davidcollom/alertmanager-github-receiver:local .
docker run --rm davidcollom/alertmanager-github-receiver:local --help
```

The image has no shell and runs `/github_receiver` directly. Supply GitHub credentials and receiver configuration according to upstream guidance, preferably from orchestrator-managed secrets.
