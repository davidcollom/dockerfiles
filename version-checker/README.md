# version-checker

This image builds the pinned Jetstack `version-checker` release and places its static binary in a non-root Alpine runtime. I use it to report container image versions in Kubernetes while keeping a multi-architecture build under the same registry namespace as the rest of the stack.

```sh
docker run --rm davidcollom/version-checker:0.2.1 --help
```

The container runs as UID 65534. Kubernetes deployment requires the API permissions described by the upstream project; grant read-only access to only the workload resources it needs to inspect.
