# system-upgrade-os-patch

A tiny Go HTTP service used by my system-upgrade automation as a moving OS patch/version signal. A request to `/version` redirects to a path shaped as `/<year>-<month>-<week>`, with weeks counted from the first Sunday of the year. Consumers can treat a changed redirect target as a new periodic version.

```sh
docker run --rm -p 8080:8080 davidcollom/system-upgrade-os-patch:0.0.0.1
curl -i http://localhost:8080/version
```

The service listens on `:8080` and has no configuration or persistent state. Its result depends on the container's current clock/timezone and is a scheduling token, not an operating-system security assessment.
