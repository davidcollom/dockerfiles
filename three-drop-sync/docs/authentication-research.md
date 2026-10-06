# 3Drop authentication research — 6 October 2026

## Verified public frontend evidence

- `3830-f1153ae91e47d8bc.js`, module 32537, initialises Better Auth's React client with `baseURL: window.location.origin`, one plugin, and `refetchOnWindowFocus: false`.
- The plugin is module 61531 from `7250-25102364527fb84e.js`. It explicitly declares `id: "api-key"`, `version: "1.7.6"` and POST methods for `/api-key/create`, `/api-key/delete`, and `/api-key/delete-all-expired-api-keys`. This is a genuine API-key lead, rather than an inference based on generic library support.
- Login buttons call `signIn.social({provider: "google"|"apple", callbackURL: window.location.origin})`.
- The auth library defaults to `/api/auth`. Its session logic calls GET `/get-session` and, if `needsRefresh` is returned, POST `/get-session`.
- No key-management UI or `x-api-key` usage was found in currently fetched route chunks. Other routes may load additional chunks.
- No evidence of a frontend bearer, device authorisation, Expo or Firebase integration was found. Generic bearer support inside the fetch library does not establish server support.
- Account favourites calls are same-origin `fetch` requests, so normal browser cookies apply.

## Live unauthenticated endpoint evidence

- GET `/api/auth/get-session` returned HTTP200 with JSON `null`.
- GET `/api/auth/api-key/list` returned HTTP401 with JSON `{"error":"Unauthorized"}`. This verifies a server-side key-list route exists, but says nothing about key permissions or acceptance on favourites endpoints.
- GET `/api/auth/device/code` and `/api/auth/token` returned HTTP404. These do not establish a usable device or JWT flow.

## Best next verification: native API keys

Better Auth documents client-managed key creation/listing and a default `x-api-key` header. Session-backed account endpoints only work with API keys if the server enables `enableSessionForAPIKeys` or explicitly verifies keys in its handlers. Client plugin presence does **not** prove either server configuration.

Once the user has signed in normally, first inspect whether 3Drop exposes API-key management. If it does, create a dedicated NAS key through that interface, write it directly to a secret file without printing it, then verify an account read endpoint using that key. If no UI exists, verify whether its documented Better Auth key routes are enabled; do not claim key creation or unattended sync works before that check.

Suggested Go config: `api_key_file` (preferred), `api_key_header` default `x-api-key`; use file mounted as Docker/Kubernetes secret. Keep credentials confined to three-drop.com; strip them from cross-host download requests and redirects. Do not put secret values in command-line flags, logs, manifests or metadata backups.

## Cookie fallback

If API keys are unavailable, a user-directed login in a dedicated browser followed by an explicit session-file import can establish a Go cookie jar. Do not extract existing browser profiles or secretly harvest cookies. Standard browser JavaScript cannot access HttpOnly session cookies, so a bookmarklet is not a viable generic solution. Refresh via the auth session endpoint, persist changed Set-Cookie values atomically with mode 0600, and report reauthentication required on expiry/revocation. Better Auth's documented default expiry is seven days with renewal, but 3Drop's actual configuration is unknown. A local callback URL may not be a trusted redirect; do not promise an OAuth loopback flow without testing.

## Mobile authentication

Official Google Play listing identifies developer HenryWu / WU FU-HUNG and package `com.threedrop.three_drop`. Store description supports cross-platform favourites/collections and cloud sync, but does not document auth protocol. No public source repository, official APK download, API documentation or mobile token contract was discovered. Do not assume a Google ID token, mobile token, website cookie or API key is interchangeable. No proprietary binary was downloaded or executed.

## Primary sources

- https://play.google.com/store/apps/details?id=com.threedrop.three_drop
- https://three-drop.com/privacy
- https://www.privacypolicies.com/live/f3cd6d4f-3280-4d92-9386-690a58675a4f
- https://better-auth.com/docs/plugins/api-key
- https://better-auth.com/docs/plugins/api-key/advanced
- https://better-auth.com/docs/concepts/session-management
- https://better-auth.com/docs/concepts/cookies
- https://better-auth.com/docs/plugins/bearer
- https://better-auth.com/docs/integrations/expo

## Research limits

Website sign-in could not complete in the parent's managed browser due to redirect connection failure. No authenticated request or credential handling was performed by this research agent. Unauthenticated API probe outcomes are recorded above. The settings page could not be fetched through web retrieval; a direct fetch did not complete promptly. No settings-specific API-key UI was verified.
