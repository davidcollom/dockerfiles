# hub3-exporter

A Go Prometheus exporter written for Virgin Media/UPC Hub 3 cable modems. It calls the modem's local `/walk` endpoints, decodes DOCSIS OIDs, and publishes downstream frequency, power and SNR plus upstream power. This project exists because those useful line-health values are otherwise trapped in the modem UI.

The default metrics endpoint is `:9463/metrics` and the default modem address is `192.168.100.1`.

```sh
docker run --rm -p 9463:9463 davidcollom/hub3-exporter:0.0.1 \
  -modemIP=192.168.100.1 -timeout=5s
```

Useful flags are `-web.listen-address`, `-web.telemetry-path`, `-modemIP`, `-timeout`, and `-Version`. The container needs network reachability to the modem management address. It runs as `nobody` and stores no persistent data.
