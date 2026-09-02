# rsync

A deliberately minimal Alpine image with `rsync` as its entrypoint. It gives Kubernetes Jobs and containerized backup workflows a predictable rsync binary without adding a general-purpose toolset.

```sh
docker run --rm -v "$PWD/source:/source:ro" -v "$PWD/dest:/dest" \
  davidcollom/rsync:0.0.1 -av --delete /source/ /dest/
```

Arguments are passed directly to rsync. Options such as `--delete` can remove destination data; validate mounts and use `--dry-run` before destructive synchronization.
