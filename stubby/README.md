# stubby

This image builds Stubby from the pinned `getdns` source release with only the stub resolver enabled, then copies it into a slim Debian runtime with its TLS/YAML dependencies. I use it as a small, multi-architecture DNS-over-TLS forwarder with a version I control.

Stubby listens on the addresses configured in its YAML configuration; the image documents port `8053`.

```sh
docker run --rm -p 8053:8053/tcp -p 8053:8053/udp \
  -v "$PWD/stubby.yml:/etc/stubby/stubby.yml:ro" \
  davidcollom/stubby:1.7.3 -C /etc/stubby/stubby.yml
```

Review upstream resolver privacy/authentication settings and pin trusted upstream DNS servers as appropriate.
