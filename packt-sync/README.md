# packt-sync

A small Go utility that authenticates to Packt, enumerates the books entitled to an account, and concurrently downloads missing formats into one directory per title.

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `USERNAME` | none | Packt account username |
| `PASSWORD` | none | Packt account password |
| `DOWNLOAD_PATH` | working directory | Library destination |
| `EXTENSIONS` | `epub,mobi,pdf` | Comma-separated formats |
| `DOWNLOAD_CONCURRENCY` | `4` | Maximum simultaneous downloads |
| `LOG_LEVEL` | `INFO` | Set to `DEBUG` for verbose logging |

```sh
docker run --rm -e USERNAME -e PASSWORD \
  -v "$PWD/books:/books" -e DOWNLOAD_PATH=/books \
  davidcollom/packt-sync:0.1.0
```

Downloads are streamed to temporary files and atomically renamed once complete. The utility uses Packt's service APIs, which may change independently. Supply credentials through a secret manager and ensure your downloading complies with the account terms.
