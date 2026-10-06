# Download-provider investigation (6 October 2026)

## Verified current 3Drop behaviour

Public frontend schemas define generic files with id, name, website, optional fileSize/filePath/type/extra/children. This does not establish a downloadable file service.

A successful public HTTP GET of https://three-drop.com/model/printables/1631299 returned HTML containing Next.js `NEXT_REDIRECT;replace;https://www.printables.com/model/1631299;307;`. The current page redirected upstream rather than providing full model/files metadata. Search-index model-detail pages are older and cannot establish today's endpoint behaviour.

## Verified minimal Printables adapter

Fetched the public source https://www.printables.com/model/1349760-api-test-kit-organizer/files successfully without credentials. The response contains `script[type=application/json][data-sveltekit-fetched]` entries. Each JSON envelope's body is a JSON string holding GraphQL responses. `data.model` entries contain complete metadata and, separately, `stls`, `slas`, `otherFiles` with file ID/name/fileSize/folder; binary URLs are not present.

Current official frontend release `7b05775272be49323d7a55c75983192c8dbe6376`:
- `/_app/immutable/chunks/2.qRMHICtN.js`: public file-list query schema.
- `/_app/immutable/chunks/2.BIkubweE.js`: download processing. Free model download is possible without `me`; occasional login prompts are suggestions. Provider `paymentRequired` and `eduLocked` flows are separate.
- `/_app/immutable/chunks/2.CSs2KARD.js`: actual GetDownloadLink mutation.
- `/_app/immutable/chunks/2.B9xNFpCR.js`: enums `stl`, `sla`, `other`, `pack`; sources `model_detail`, `model_viewer`.

Observed request:

POST https://api.printables.com/graphql/
Content-Type: application/json
Origin: https://www.printables.com

```graphql
mutation GetDownloadLink($id: ID!, $modelId: ID!, $fileType: DownloadFileTypeEnum!, $source: DownloadSourceEnum!) {
  getDownloadLink(id: $id, printId: $modelId, fileType: $fileType, source: $source) {
    ok
    errors { field messages }
    output { link count ttl }
  }
}
```

Variables for one ordinary free file: id=5683165, modelId=1349760, fileType=stl, source=model_detail. Response: `ok:true`, valid HTTPS link on `files.printables.com`, TTL=86400. No authentication was required to generate the URL. This mutation increments the normal download count and must not be used in dry runs.

**Important verification boundary:** one ordinary GET of the returned file link returned HTTP403 from the execution environment. No retries, header impersonation, proxying or challenge bypass were attempted. Link discovery is verified; real STL byte retrieval remains unverified/blocked here. A user-side/NAS run must establish whether the normal download succeeds. Do not claim end-to-end downloads work based solely on link discovery.

## Implemented provider

`dockerfiles/three-drop-sync/printables.go` provides `DiscoverPrintables(ctx, []Model, PrintablesOptions{MaxFiles,Output})`. Public page fetch + derived GraphQL mutation only. Production discovery serialises requests with at least 1 second delay. Strict numeric model/file IDs, file-ID-prefixed safe names, no 3Drop-cookie sharing, HTTPS file host allowlist, restricted/unsupported/failed/limited counters. Conservatively skips all non-null price, club and educational projects. Verifies existing local files before generating links; limits new link batches. Signed URLs stay only in memory and are excluded from JSON summary.

Mocked tests cover extraction, exact mutation variables, max-file limit before links, auth header separation, access denial, HTTP429, changed HTML schema, GraphQL errors, restricted responses and unexpected download URL hosts. Provider tests passed.

## Other sites

Cults official primary docs https://cults3d.com/en/api and https://cults3d.com/es/pages/graphql explicitly state public API supplies metadata, not other users' 3D files. GraphQL uses POST https://cults3d.com/graphql and HTTP Basic Auth (nickname + API key). Official FAQ https://cults3d.com/en/questions says ordinary file downloads require login. Separate normal account integration is necessary; do not assume Cults API yields download files.

Thingiverse official developer portal and API libraries exist: https://www.thingiverse.com/developers/libraries and https://www.thingiverse.com/developers/getting-started. Official REST reference fetch returned HTTP403; did not retry. Community SDK describes GET /things/{id}/files and GET /files/{id}/download with bearer token, but this is not adequate primary verification of today's working interfaces and auth flow. No borrowed embedded API tokens, user cookies or guessed endpoint probes were used.

MakerWorld/MakerOnline remain unsupported rather than falsely claiming metadata refs suffice for binary downloads.
