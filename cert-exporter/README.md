# cert-exporter

This image builds a pinned release of `joe-elliott/cert-exporter` and runs it as UID/GID 1000 in a small Alpine image. I use it to monitor certificate expiry while retaining control of the upstream version and runtime user.

```sh
docker run --rm davidcollom/cert-exporter:2.3.2 --help
```

Mount only the certificate paths that need monitoring and pass the upstream command-line options. The runtime user must have read permission on those files; private keys are not normally required for expiry monitoring and should not be mounted unnecessarily.
