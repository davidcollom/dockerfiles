# unifi-cert-updater

A Go utility written to automate TLS certificate rotation on UniFi OS. It reads `tls.crt` and `tls.key` from a Kubernetes TLS Secret, authenticates to UniFi, compares certificate fingerprints, uploads only when needed, activates the selected certificate, and removes old certificates above a configurable retention limit.

The program performs one reconciliation and exits, so it is intended for a Kubernetes CronJob or other scheduler.

| Environment variable | Required | Purpose |
| --- | --- | --- |
| `UNIFI_API_URL` | yes | UniFi OS/controller base URL |
| `UNIFI_USERNAME` | yes | Local UniFi login username |
| `UNIFI_PASSWORD` | yes | Local UniFi login password |
| `NAMESPACE` | yes | Namespace containing the TLS Secret |
| `SECRET_NAME` | yes | TLS Secret name |
| `MAX_CERTS` | no | Certificates to retain; default `5` |
| `LOG_LEVEL` | no | `debug`, `info`, `warn`, or `error` |

```sh
docker run --rm \
  -e UNIFI_API_URL=https://unifi.example.net \
  -e UNIFI_USERNAME -e UNIFI_PASSWORD \
  -e NAMESPACE=networking -e SECRET_NAME=unifi-tls \
  -v "$HOME/.kube:/root/.kube:ro" \
  davidcollom/unifi-cert-updater:0.0.2
```

The Kubernetes identity needs `get` access to the named Secret. UniFi credentials need certificate-management access. The client currently accepts an unverified TLS certificate when connecting to UniFi, which is useful during bootstrap but means network placement and credential handling are important. Test with non-critical equipment before relying on pruning behavior.

The reusable API client lives in `pkg/unifi`; its separate README describes development usage.
