# Cloud-browser reference export

This is an operator runbook and DOM-only helper, not an unattended scheduled task.
It keeps 3Drop authentication in the signed-in browser. The NAS receives a private
JSON inventory of references; its provider adapters resolve fresh download links.
No Google token, session cookie, provider credentials or signed download URL belongs
in the manifest. Do not commit an account export to this repository.

1. Select the existing 3Drop cloud-browser tab through the supported `cua` API.
   Read its advertised documentation. Use `browserAuth` for Google sign-in when
   required, with the user completing passkey or other verification through the
   handoff. Do not inspect browser cookies, local storage or hidden app state.
2. Confirm the signed-in rendered pages and the current selectors against a fresh
   DOM snapshot. The helper reflects the observed English website layout:
   `main h1`, nearby `(count)`, model `h3` cards with a provider icon and external
   title link, `Like`/`Unlike` buttons, collection `h3` headings with `N items`, and
   `a[aria-label="Next page"]`. A changed layout must be reviewed, not guessed.
3. Load the contents of `browser-export.js` into the `cua_repl` runtime through the
   supported operator code-loading workflow. This file defines functions; it does
   not select a browser or perform login. With the selected tab bound as `tab`, run:

   ```js
   const result = await exportThreeDrop(tab, {
     savedUrl: 'https://three-drop.com/favorites/saved',
     likesUrl: 'https://three-drop.com/favorites/likes',
     collectionsUrl: 'https://three-drop.com/favorites/collections',
   });
   nodeRepl.write({ complete: result.manifest.complete, totals: result.manifest.totals });
   ```

   These URLs were observed on the website. The helper follows rendered pagination
   links and clicks collection headings from a newly retrieved `.all()` list on
   each return to the index. Duplicate names stay separate through their actual
   UUID detail URLs. Models present only in collections are included.
4. If `result.error` is non-null, stop. Save the partial result privately for
   diagnosis if needed, keeping `complete: false`. Never publish it to the active
   Drive file. A browser safety block must not be retried, bypassed, or worked
   around using another browser/network path. Layout changes, unknown provider URL
   routes, missing pages, duplicate references and account count changes also fail
   closed. Strict URL ID extraction currently supports Printables, Thingiverse and
   MakerWorld; other providers require observed route support before export.
5. Save `JSON.stringify(result.manifest, null, 2)` as a private local JSON file only
   after `complete === true`. Validate it with the NAS CLI before publication. JSON
   is canonical: it preserves memberships, duplicate collection names, counts and
   completeness that a flat CSV cannot represent safely.
6. Publish to a dedicated **private** Google Drive file. Use an available Drive
   connector only if it actually supports creating/updating a file from the exact
   JSON bytes, and verify the returned file ID and content. A read-only search
   connector cannot publish. Alternatively, use the Go CLI's explicit Drive
   publishing command with local Google credentials and the stable file ID. Never
   make a public link, use a title search as file identity, or overwrite the active
   file with an incomplete export. Keep the file ID stable across updates so the
   NAS can fetch the same object and validate it before importing/downloading.
7. Configure the NAS with a separate Drive reader credential and that file ID.
   Schedule the NAS pull/import/download command using its documented flags for
   worker count, maximum files and maximum file size. Keep provider login separate
   from Drive and 3Drop sessions. A complete inventory does not imply every provider
   supports downloads.

Do not create an automatic cloud-browser export task until session reuse, complete
pagination, the private Drive write path and one NAS pull are verified end to end.
Interactive cloud-browser authentication may need the user again after expiry.
Count checks reduce accidental omissions; they cannot give a transactional snapshot
if equal-count changes occur while the account is being captured.

Synthetic tests exercise orchestration and strict provider URL parsing:

```sh
node --test agents/browser-export.test.js
```

These tests are not live account or DOM verification. Actual account export remains
subject to the website layout and the browser's supported action budget.
