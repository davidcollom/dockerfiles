# three-drop-sync

A Go service for backing up 3Drop likes and collections to a mounted NAS, with
bounded concurrent model downloads. It uses Cobra for the CLI and Viper for
configuration.

**Experimental:** authenticated 3Drop account access has not yet been verified.
Automatic file discovery currently targets free Printables models; other source
sites remain reference-only. A generated Printables download link was verified,
but fetching its file returned HTTP 403 in the research environment. This is not
yet a verified complete backup of a real account.

## Commands

```sh
go build -o /tmp/three-drop-sync .

# Validate account references without writing to the NAS.
/tmp/three-drop-sync sync --cookie-file=/private/session.cookie --dry-run

# Archive likes/collections and download supported files.
/tmp/three-drop-sync sync --cookie-file=/private/session.cookie \
  --output=/mnt/nas/3drop --download --workers=4 \
  --max-files=50 --max-file-size=1GiB

# Download supported models from an existing archive.
/tmp/three-drop-sync download --from-library --output=/mnt/nas/3drop \
  --workers=4 --max-files=50 --max-file-size=1GiB

# Alternatively download explicit authorised direct links.
/tmp/three-drop-sync download --manifest=/private/model-links.json \
  --output=/mnt/nas/3drop --workers=4 --max-files=50

# Import saved JSON responses from the two 3Drop account endpoints.
/tmp/three-drop-sync import --input=/private/export --output=/mnt/nas/3drop
```

| Flag | Default | Purpose |
| --- | --- | --- |
| `--config` | unset | Explicit YAML, JSON or TOML configuration file |
| `--output` | `/library` | NAS archive destination |
| `--cookie-file` | unset | Website session secret file, for `sync` |
| `--download` | `false` | Also download supported files during `sync` |
| `--input` | unset | Saved account JSON directory, for `import` |
| `--manifest` | unset | Explicit direct-link JSON array, for `download` |
| `--from-library` | `false` | Discover supported files from the current archive |
| `--workers` | `4` | Concurrent file transfers, between 1 and 128 |
| `--max-files` | `0` | Maximum new file attempts per run; zero means unlimited |
| `--max-file-size` | `1GiB` | Size per file: bytes or integer KiB/MiB/GiB |
| `--retries` | `2` | Retries for temporary transfer failures, maximum 10 |
| `--interval` | `0s` | Run once; `sync`/`import` can repeat at intervals >=1m |
| `--dry-run` | `false` | Validate/count references without writing files or generating download links |

For `download`, choose exactly one of `--manifest` and `--from-library`.
Configuration precedence is **flags > THREE_DROP_* environment > config file >
defaults**. Environment keys replace hyphens with underscores, e.g.
`THREE_DROP_MAX_FILES=50` and `THREE_DROP_COOKIE_FILE=/run/secrets/session.cookie`.
Config keys use the flag names: `output`, `workers`, `max-files`, etc.
See `examples/config.yaml`. Secret values belong in mounted files, never config
values or command-line arguments.

An explicit download manifest is a JSON array:

```json
[
  {
    "website": "printables",
    "model_id": "123456",
    "name": "model.stl",
    "url": "https://downloads.example.org/model.stl"
  }
]
```

Replace the example URL with an authorised direct file URL. An optional `sha256`
field verifies a known checksum. URLs may contain temporary download signatures;
keep manifests private. URL credentials and non-public network destinations are
rejected. The downloader does not send the 3Drop session to file hosts.

## Archive behaviour

The account reader uses two GET endpoints observed in the website's public
frontend on 6 October 2026:

- `/api/favorites/likes/check-batch`: a `liked` array of `website:externalId` keys.
- `/api/favorites/collections/item-refs`: collections and their model references.

These are private service interfaces, not a documented public API. Responses are
validated together before committing the backup. Missing fields, rejected
sessions and partial requests fail visibly. Both are fetched sequentially, so
edits during a run may appear on the next sync.

Each changed response pair produces an immutable SHA-256-addressed directory
under `snapshots/`, containing raw `likes.json`, raw `collections.json` and a
deduplicated `catalogue.json`. `latest.json` points to the current snapshot.
Removing a like or collection never deletes old snapshots or downloaded files.
JSON object-key order/whitespace changes do not create snapshots; array reordering
can. JSON import accepts these same response shapes, **not an unverified Excel
export format**.

Downloads are streamed to temporary files with bounded workers, size limits and
SHA-256 checks. Completed files use content-addressed paths under `downloads/`;
`current.sha256` points to a file version. Known different content versions are
retained. Verified local files are skipped without consuming `--max-files`.
Without a supplied expected checksum, skipping verifies local integrity and does
not detect remote file revisions. Corrupt content is retained for inspection and
repaired by a new transfer. Unsupported source sites are reported explicitly.

Requests use bounded retries and respect server retry delays; a long requested
delay fails the run for a later scheduled retry. Source discovery is conservative
and sequential; the worker limit applies to file transfers. File/directory syncing,
atomic renames and process locks protect the NAS archive against partial or
concurrent runs. The filesystem must support locking, directory fsync and atomic
renames. Include this directory in your NAS snapshot/backup policy.

With `--interval`, run failures are logged and retried at the configured interval,
so a temporary provider failure does not stop daily syncs or trigger immediate
restarts. One-shot commands exit non-zero on failures.

## Provider interfaces

`Provider` in `providers.go` defines a source name and a `Discover` method that
returns files and explicit existing/restricted/unsupported/limited/failed counts.
The registry groups saved references by source and applies one deterministic
file budget across adapters. Each adapter owns its account handling and request
pacing; 3Drop credentials are not shared with hosting providers.

Printables is the first implementation. Add an adapter and register it without
changing the archive format, CLI or transfer engine. Discovery currently runs
sequentially; the shared worker pool performs file transfers concurrently. This
keeps provider request limits separate from the download-worker limit.

## Authentication

The website uses Better Auth with Google/Apple sign-in. API-key functionality was
found in its public client and a server-side key-list route, but key access to
favourites is **unverified**. It is a promising future unattended-login option,
not an implemented credential mode. See `docs/authentication-research.md`.

The current bootstrap mode takes an existing website session in a local secret
file. In your own signed-in browser's developer tools, find a request to one of
the account endpoints and save its **Cookie header value only** to a private file
outside this repository. Do not include the `Cookie:` label, whole requests or a
HAR. Do not paste the value into chat, commit it, or put it in logs.

The session file is re-read on each run, but automatic cookie renewal is not yet
implemented. Renew it locally when the session expires. Google/Apple passwords
are never required by this utility. Mobile authentication and app/web token
interchangeability remain unverified. An authenticated account run, including
response completeness and visibility of app collections, is still needed.

## Containers and scheduling

The existing repository workflows discover `Dockerfile`, `VERSION` and
`PLATFORMS`. On merge to `main`, they publish versioned AMD64/ARM64 images to
GHCR and Docker Hub using the configured repository credentials. A draft PR does
not publish an image.

```sh
docker build -t three-drop-sync:local .
docker run --rm --read-only --cap-drop=ALL \
  --security-opt=no-new-privileges:true \
  -v /private/session.cookie:/run/secrets/session.cookie:ro \
  -v /mnt/nas/3drop:/library \
  three-drop-sync:local sync --cookie-file=/run/secrets/session.cookie \
  --download --max-files=50 --workers=4
```

The default UID/GID is `10001:10001`. Prepare the NAS directory and session file
with matching permissions or use `--user` with the NAS UID/GID. Files use mode
`0640` and archive directories `0750`; the container root filesystem can be
read-only while the NAS mount remains writable.

`examples/compose.yaml` uses `LIBRARY_PATH`, `COOKIE_FILE` and optional
`PUID`/`PGID`. While the image is unpublished, run
`docker compose -f examples/compose.yaml up --build`.
`examples/cronjob.yaml` starts suspended and expects a same-namespace
`three-drop-library` PVC and `three-drop-session` Secret. Verify an account dry
run and a small real transfer before enabling scheduling. No deployment or live
NAS connection has been performed here.

## Development and remaining work

```sh
go test -race -cover ./...
go vet ./...
```

Follow [issue #69](https://github.com/davidcollom/dockerfiles/issues/69) for account
verification, durable authentication, more source adapters, model metadata,
creator/licence/preview preservation, provider rate limits and real NAS restore
checks. Research evidence is in `docs/`; fixtures are synthetic and test only
locally observed contracts. Container publishing awaits the integration review.
