# Android session authentication investigation

Inspected on 6 October 2026. This is static evidence, not a verified live authentication implementation.

## Package provenance

Downloaded the XAPK through APKPure's public download link. The package manifest identifies `com.threedrop.three_drop`, version `1.11.5`, version code `554`. XAPK SHA-256: `16a6548b6063899995072e768b21742ca21840d4b3c65920b5a93e5cca08d3fe`. The publisher signing certificate has not been independently verified against a Play-installed copy. No package was installed or executed. No account credential was extracted.

Inspected printable strings in the ARM64 Flutter AOT binary `lib/arm64-v8a/libapp.so` inside `config.arm64_v8a.apk`. Strings and symbol names are evidence of capabilities and code structure; adjacency in the string pool does not establish a call graph.

## Direct evidence

The binary includes `package:three_drop/repositories/auth_repository.dart` and these diagnostic strings:

- `AuthRepository Google sign-in: launching google_sign_in.authenticate`
- `AuthRepository Google sign-in: missing Google ID token`
- `AuthRepository Google sign-in: exchanging Google ID token with Better Auth`
- `AuthRepository Google sign-in: Better Auth response status `
- `AuthRepository Google sign-in: session refresh completed, authenticated=true`
- `AuthRepository Google sign-in: session refresh completed without authenticated user`
- `AuthRepository refreshSession error: `

AuthRepository symbols sharing the Dart library suffix include `_refreshSession`, `_buildCookieHeader`, `_handleSetCookie`, `_splitSetCookieHeader`, `_saveCookies` and `_clearAuthData`. The binary contains `auth_cookie_json`, `session_cookies` and `set-cookie` strings. The exact storage implementation and cookie names remain unverified.

Embedded 3Drop endpoints:

- `/api/auth/sign-in/social`
- `/api/auth/get-session`
- `/api/auth/sign-out`
- `/api/favorites/likes`
- `/api/favorites/collections`
- `/api/favorites/follows`
- `/api/favorites/v2/sync`

Public, unauthenticated GET checks returned HTTP 200 with JSON `null` from `/api/auth/get-session` and HTTP 401 with an unauthorised error from `/api/favorites/collections`.

A prior unauthenticated POST with an empty JSON body to `/api/auth/device/code` returned HTTP 404. Standard OIDC discovery paths also returned 404. This does not rule out another auth base URL or a separate mobile service, and does not prove whether the bearer plugin is enabled.

## Interpretation and exclusions

The strongest supported hypothesis is native Google authentication producing an ID token, exchanged with Better Auth, followed by persisted cookie-based session requests. This remains a hypothesis about the complete runtime sequence until verified using a live authorised session.

The method name `_refreshSession` does not prove rolling expiry or automatic reauthentication. It may only retrieve current session state. The user's successful app sync after a long gap supports persistent or silently renewed authentication, but does not establish token lifetime.

The generic message `Session cookie token exchange successful` appears alongside an `ajax/user/exchange_session_for_token` string somewhere in the binary. It may belong to a hosting-provider integration. It must not be used as evidence of a 3Drop token-exchange endpoint. Likewise, bearer/access-token/refresh-token strings may belong to Printables, Thingiverse or other integrations.

## Next verification and implementation

1. Obtain a session through a local user-completed 3Drop sign-in; keep secrets out of chat, logs and this repository. The prior cloud browser could not provide supported session export and encountered a security-policy block; do not bypass that restriction.
2. Inspect the authenticated session response for expiry and cookie updates, redacting token and personal fields.
3. Verify a single page of likes and collections, including pagination, collection IDs and unliked collection members. Do not invoke `/favorites/v2/sync` until its method and mutation semantics are established.
4. Implement local browser-assisted login and a restricted credential file or operating-system credential store. The current Go client only reads a static Cookie header file; it does not retain response cookie updates.
5. Persist legitimate session cookie updates atomically, with restrictive permissions, origin scoping and no credential forwarding to model providers. Test expiry, rotation, rejection and restart behaviour before claiming unattended renewal works.
6. Keep PR #70 draft until live account sync and actual model file transfer succeed.
