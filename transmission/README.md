# transmission

An opinionated Transmission daemon image for my media stack. It provides persistent defaults, Tini signal handling, runtime UID/GID remapping, timezone configuration, environment-based settings overrides, and automatic blocklist refresh.

Persist `/var/lib/transmission-daemon`. The web/RPC interface uses port `9091`; peer traffic uses `51413/tcp` and `51413/udp`.

```sh
docker run -d --name transmission \
  -p 9091:9091 -p 51413:51413/tcp -p 51413:51413/udp \
  -v transmission-data:/var/lib/transmission-daemon \
  -e USERID=1000 -e GROUPID=1000 -e TZ=Europe/London \
  -e TRUSER=admin -e TRPASSWD='change-me' \
  davidcollom/transmission:latest
```

Any `TR_*` variable is converted to a lowercase, hyphenated Transmission setting. `TRUSER` and `TRPASSWD` control startup authentication; `BLOCKLIST=no` disables blocklist download. Pass `-n` to disable authentication only on trusted networks. Change the example password and do not expose RPC directly to the internet.
