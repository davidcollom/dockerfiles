# kubectl

This image turns Rancher's version-matched `k3s-upgrade` image into a convenient `kubectl` command container. It keeps the client version aligned with my k3s fleet and works on the same set of architectures without maintaining another binary-download path.

The entrypoint is `/opt/k3s kubectl` and the default command is `--help`.

```sh
docker run --rm -v "$HOME/.kube:/root/.kube:ro" \
  davidcollom/kubectl:1.30.3 get nodes
```

Mount kubeconfig read-only and use narrowly scoped credentials. The version maps to an upstream tag shaped like `v<VERSION>-k3s1`.
