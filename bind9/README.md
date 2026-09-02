# bind9

This is a container-friendly BIND 9 image used for authoritative or recursive DNS in my environment. The customization creates and repairs ownership of `/var/cache/bind`, then starts `named` in the foreground as the `named` user so logs and lifecycle signals work correctly under Docker/Kubernetes.

Ports `53/tcp`, `53/udp`, `953`, and `8053` are exposed. Mount configuration at `/etc/bind` and persistent writable cache/zone data at `/var/cache/bind`.

```sh
docker run --rm --name bind9 \
  -p 53:53/tcp -p 53:53/udp \
  -v "$PWD/bind:/etc/bind:ro" \
  -v bind-cache:/var/cache/bind \
  davidcollom/bind9:latest
```

Review recursion, transfer, update, and control-channel ACLs before exposing the service outside a trusted network.
