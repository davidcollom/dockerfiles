# Google Drive hand-off

The browser agent exports a JSON reference inventory from the rendered signed-in
3Drop pages. Google Drive carries that inventory; the Go process on the NAS
validates it, archives references and resolves fresh model-file links using its
provider adapters. The NAS does not need the browser's 3Drop session.

JSON is the canonical format. A flat CSV does not reliably represent multiple
collection memberships, duplicate collection names or completion checks. This
implementation does not accept CSV or Google Sheets documents: use a regular
JSON file in Drive. It does not store STL/3MF files on Drive.

## Manifest contract

```json
{
  "schemaVersion": 1,
  "source": "three-drop",
  "capturedAt": "2026-10-06T22:00:00Z",
  "complete": true,
  "totals": {"likes": 1, "collections": 1, "savedModels": 1},
  "models": [{
    "website": "printables", "externalId": "123",
    "title": "Example model", "sourceUrl": "https://www.printables.com/model/123",
    "liked": true, "collectionIds": ["collection-id"]
  }],
  "collections": [{"id": "collection-id", "name": "Examples", "itemCount": 1}]
}
```

This is synthetic example data, not a real model export. The producer must verify
every page and collection against the rendered totals. The consumer requires an
explicit complete flag and counts, unique model/collection IDs and exact
membership counts. Query strings, fragments, credentials and signed file URLs
are rejected in source URLs. A blocked or interrupted export must remain
`complete: false` and must never replace the active Drive object. Count checks
cannot detect every equal-count change during capture; avoid editing the account
while exporting.

The DOM helper currently extracts IDs only for the verified Printables,
Thingiverse and MakerWorld URL patterns. Other provider routes stop export with
an incomplete result. The Go schema recognises additional known provider URLs,
but NexPrint is deliberately rejected pending route verification. Only
Printables currently has automatic file discovery.

## Export and publish

Follow [the browser agent runbook](../agents/README.md). This is an operator-driven
browser workflow; it is not a configured ChatGPT scheduled task. Session reuse,
website layout, complete export and Drive writes still require live verification.

Validate locally without downloading or writing the NAS:

```sh
three-drop-sync handoff --handoff-file catalogue.json --dry-run --download
```

With an installed Drive connection that supports file writes, an agent can upload
the validated JSON bytes into a dedicated private Drive file. Connector support
and permissions must be verified; a search-only connection is insufficient.
The CLI also supports first publication and subsequent updates:

```sh
three-drop-sync drive publish --handoff-file catalogue.json \
  --drive-credentials-file /private/drive-publisher.json
# Prints the new file ID. Keep this ID for the NAS and all future updates.

three-drop-sync drive publish --handoff-file catalogue.json \
  --drive-file-id YOUR_FILE_ID \
  --drive-credentials-file /private/drive-publisher.json
```

`--drive-folder-id` and `--drive-name` select the parent and name for creation.
If `--drive-file-id` is omitted on every run, each publication creates another
file. Folder permissions are inherited: use a private folder. This service does
not change sharing permissions. Uploads are bounded to 32 MiB and nonresumable;
after an uncertain create failure, check Drive before retrying to avoid duplicate
files. Publication runs once; `--dry-run` validates and does not obtain credentials
or upload. Publication rejects inventories older than seven days or more than
five minutes in the future.

## Drive credentials

Drive authentication is separate from Google sign-in to 3Drop. Configure your own
Google OAuth application and enable the Drive API. Provision a private
authorised-user credential file with `client_id`, `client_secret`, `refresh_token`
and optionally `type: authorized_user` and
`token_uri: https://oauth2.googleapis.com/token`. Do not use credentials embedded
in another application. No interactive Google login helper is included here.

One supported way to obtain this file is Google's `gcloud` tool, using your own
downloaded **desktop OAuth client** JSON. Use a separate gcloud configuration
directory so existing Application Default Credentials are not overwritten:

```sh
mkdir -m 700 /private/three-drop-drive
CLOUDSDK_CONFIG=/private/three-drop-drive gcloud auth application-default login \
  --client-id-file=/private/desktop-oauth-client.json \
  --scopes=https://www.googleapis.com/auth/drive.file
chmod 600 /private/three-drop-drive/application_default_credentials.json
```

Complete Google's consent flow locally. `drive.file` works for files created by
or explicitly granted to that OAuth application. A JSON file created by a
different connector/application is not automatically accessible with this scope.
For a separate reader app without per-file access, `drive.readonly` can read
accessible Drive files, but grants wider read access: select that scope knowingly
when provisioning the reader. A file ID alone grants no permission. Do not make
the catalogue publicly accessible as a workaround.

Use `--drive-credentials-file` for unattended operation. Each poll obtains a fresh
access token using the refresh token; secret values are not written to logs or the
NAS catalogue. Consent revocation, OAuth application policy or token expiry can
still require reauthorisation. If Google unexpectedly returns a changed refresh
token, the process fails and requires reprovisioning rather than silently
discarding it or modifying a mounted secret. A Google OAuth application in
Testing can have short-lived refresh tokens; configure the intended consent
status and policy before treating this as an unattended deployment.

For temporary checks, `--drive-token-file` accepts an access token. It cannot renew
itself. Set exactly one credential source. Secret files must be regular files with
no group/other permissions (for example 0600), owned/readable by the container
UID; symlinks are rejected. Bind-mounted Kubernetes secret-volume symlinks cannot
be read directly: use a private regular-file copy made by an init container, or
an appropriately configured regular-file secret mount. Docker Compose local
secret mounts retain host ownership: match PUID/PGID or provision the file for UID
10001. Keep secrets outside git.

## NAS pull and download

```sh
three-drop-sync drive sync --drive-file-id YOUR_FILE_ID \
  --drive-credentials-file /private/drive-reader.json \
  --output /mnt/nas/3d-library --download \
  --workers 4 --max-files 50 --max-file-size 1GiB --interval 6h
```

Use [compose-drive.yaml](../examples/compose-drive.yaml) or
[drive-config.yaml](../examples/drive-config.yaml). The existing repository image
publishing workflow builds this component; the new image is not published until
the branch is merged. The Compose example can build it locally.

Every poll reloads the selected file and mounted credentials. Invalid, incomplete,
stale or older inventories fail before updating the current archive. The default
age limit is 168 hours; `--max-manifest-age=0s` disables only the age limit, not
count validation or rollback protection. Failures retry at `--interval`, while
one-shot runs return a nonzero exit status. A dry run reads Drive but does not
write files or request provider download links. Repeated runs also retry downloads
even when the reference inventory has not changed.

Reference snapshots retain the original archive semantics: unlikes/removals never
delete previously downloaded files. Enriched hand-offs preserve titles and source
URLs under `handoffs/<sha256>.json`, with a separate `latest-handoff.json` pointer.
Run only one reference writer per library; do not mix the cookie sync and Drive
sync processes against the same directory. Provider authentication remains
separate, and unsupported sources are reported. Expiring download links are
resolved locally and are not persisted in Drive.

## Verification status and sources

Automated tests cover manifest completeness, counts, duplicates, URL validation,
Drive refresh/fetch/create/update, secret redaction, byte bounds, redirects,
freshness, rollback, CLI dry runs and cancellation. They use synthetic fixtures.
Live Drive publication, complete browser export and a NAS file transfer remain
unverified; do not enable a scheduled browser task before proving them.

- [Google Drive OAuth scopes](https://developers.google.com/workspace/drive/api/guides/api-specific-auth)
- [Drive file uploads](https://developers.google.com/workspace/drive/api/guides/manage-uploads)
- [Drive file download](https://developers.google.com/workspace/drive/api/guides/manage-downloads)
- [Google OAuth refresh](https://developers.google.com/identity/protocols/oauth2/web-server#offline)
- [gcloud application-default login](https://docs.cloud.google.com/sdk/gcloud/reference/auth/application-default/login)
