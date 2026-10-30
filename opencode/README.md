# OpenCode

The official OpenCode v2 image with Git, SSH client, Bash, curl, jq, ripgrep,
Python, pipx, uv, Node.js, npm, GitHub CLI, Docker CLI and Buildx for a persistent coding workspace. Runs as UID/GID 1000.
The upstream OpenCode tag is pinned in `UPSTREAM_VERSION` in the Dockerfile (currently `2.2.2`);
`VERSION` identifies this derived image. `PLATFORMS` follows the shared build workflow.
Docker tooling is client-only: no Docker daemon, host socket, or cluster credentials
are included. A separately isolated builder is required for container builds.

## Python tooling

`pipx` and `uv` come from the Alpine community repository, so they are available to
every user in the image without a Python bootstrap:

```sh
pipx install <package>       # isolated virtual environment per tool
uv tool install <package>    # the same, managed by uv
uvx <package>                # run a tool ephemerally
```

Both install executables into `~/.local/bin`, which is on `PATH` in this image.
`PIPX_HOME` and `PIPX_BIN_DIR` point at `~/.local`, so tool installs land on the
volume that is mounted at `/home/opencode` and survive a restart.

## Starting a fresh GitHub checkout

There are two ways to pull a repository into the workspace.

### The `/checkout` command

The image ships a global OpenCode command, so the checkout can be started from
inside OpenCode:

```
/checkout owner/repo
```

The command runs the bundled `opencode-checkout` helper, which clones the
repository over HTTPS into the workspace at `/workspace/github/<owner>/<repo>`
(or `$OPENCODE_CHECKOUT_ROOT` when set) and prints the absolute path of the
checkout. Private repositories are cloned with the Git credentials configured in
the container, for example a GitHub App credential helper or `gh auth setup-git`.
An existing checkout is refreshed rather than cloned again, and a checkout that
belongs to a different repository is refused instead of being overwritten.

The helper is also usable directly:

```sh
opencode-checkout owner/repo
opencode-checkout owner/repo branch
```

### Native `references`

OpenCode v2 can clone and refresh a repository itself. Declare it in
`opencode.json`:

```json
{
  "references": {
    "myrepo": {
      "repository": "owner/repo",
      "description": "Why the agent should use this repository",
      "branch": "main"
    }
  }
}
```

The `owner/repo` shorthand is enough for a public repository on GitHub; a string
value is treated as a repository unless it starts with `.`, `/` or `~`, which
select a local directory instead.

OpenCode clones the repository in the background into
`~/.local/share/opencode/repos/github.com/<owner>/<repo>` and refreshes it at
most once every 24 hours. The checkout is listed by `GET /api/reference`,
attached in the TUI with `@myrepo`, and offered to the agent as an additional
directory to read. A `description` is required for the agent to be told about
the reference.

References live on the volume mounted at `/home/opencode`, so the clone is
reused across restarts instead of being fetched again.

## Building

The repository's `Build Image` workflow publishes this image to Docker Hub and
GHCR. The two registries are pushed in separate steps, so a registry
authentication failure in one of them cannot stop the other from receiving the
image.

Pushing to GHCR needs credentials that can write to the
`ghcr.io/davidcollom/opencode` package. The workflow's `GITHUB_TOKEN` only
works when that package is linked to the `davidcollom/dockerfiles` repository
with write access; the package link alone has been observed to leave the
registry rejecting the push with `permission_denied: read_package`. To make the
GHCR push independent of that, add a repository secret:

- `GHCR_TOKEN` - a classic personal access token for `davidcollom` with the
  `write:packages` scope. When it is set the workflow uses it instead of
  `GITHUB_TOKEN`.
- `GHCR_USERNAME` - optional, defaults to the repository owner
  (`davidcollom`).

To build and push by hand instead, from the Dockerfiles repository root:

```sh
version=$(cat opencode/VERSION)
platforms=$(paste -sd, opencode/PLATFORMS)
docker buildx build --builder local --platform "$platforms" \
  --build-arg VERSION="$version" \
  -t "ghcr.io/davidcollom/opencode:$version" \
  -t "davidcollom/opencode:$version" \
  --push opencode/
```

Use the existing Docker credential helper; do not put registry tokens in this folder.
Set `OPENCODE_PASSWORD` using a secret. The OpenCode v2 HTTP username is `opencode`.
Persist `/home/opencode` on block storage (SQLite sessions/authentication), and mount
NFS project storage at `/workspace`. Configure providers through `/etc/opencode/opencode.json`.
The Kubernetes resources are in the homelab repository under `clusters/home/opencode`.

`/etc/opencode` is the system configuration directory. The image installs the
`/checkout` command there; when the directory is mounted over with a
ConfigMap, copy `command/checkout.md` into the mount so the command is still
registered.
