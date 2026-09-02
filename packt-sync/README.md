# packt-sync

A small Perl utility that authenticates to Packt, enumerates the books entitled to an account, and downloads missing formats into one directory per title. I containerized it to run the same repeatable library-sync job on a schedule without maintaining Perl modules on the host.

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `USERNAME` | none | Packt account username |
| `PASSWORD` | none | Packt account password |
| `DOWNLOAD_PATH` | working directory | Library destination |
| `EXTENSIONS` | `epub,mobi,pdf` | Comma-separated formats |
| `LOG_LEVEL` | `INFO` | Perl Log4perl level |

```sh
docker run --rm -e USERNAME -e PASSWORD \
  -v "$PWD/books:/books" -e DOWNLOAD_PATH=/books \
  davidcollom/packt-sync:0.0.2
```

The script uses Packt's service APIs, which may change independently. Supply credentials through a secret manager and ensure your downloading complies with the account terms.
