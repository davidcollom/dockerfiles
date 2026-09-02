# ssh-client

A version-pinned Alpine image containing `openssh-client`. It exists for CI jobs, Kubernetes Jobs, and automation steps that need SSH/SCP but should not inherit a larger general-purpose image.

There is no custom entrypoint, so specify the command:

```sh
docker run --rm -it -v "$HOME/.ssh:/root/.ssh:ro" \
  davidcollom/ssh-client:3.17.1 ssh user@example.net
```

Mount keys read-only, verify host keys, and avoid disabling `StrictHostKeyChecking` in unattended workflows.
