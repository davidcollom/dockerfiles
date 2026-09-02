# flexget

This image packages a pinned FlexGet release with the extra integrations used by my media automation: Node.js, SQLite, Transmission RPC, SOCKS support, and Cloudflare-oriented request helpers. A packaged Pendulum wheel is installed explicitly to keep ARM32 builds working.

Mount `flexget.yml` and FlexGet's state database under `/root`. The intended command is a one-shot execution using `/root/flexget.yml`; override the command if a daemon or different subcommand is required.

```sh
docker run --rm -v "$PWD/flexget:/root" davidcollom/flexget:3.19.26 \
  flexget -c /root/flexget.yml execute
```

The image sets `TZ=Europe/London`. Feed passwords and API tokens through environment/secret injection rather than storing them in a public configuration file.
