# ddclient

A small, pinned Alpine wrapper around `ddclient`. It exists so dynamic-DNS updates use the same version and image naming convention as the rest of my homelab images.

The entrypoint is `ddclient`; provide a configuration file and append any required arguments.

```sh
docker run --rm \
  -v "$PWD/ddclient.conf:/etc/ddclient/ddclient.conf:ro" \
  davidcollom/ddclient:v3.11.2 -foreground -verbose
```

The configuration commonly contains provider credentials. Keep it out of source control and restrict its file permissions.
