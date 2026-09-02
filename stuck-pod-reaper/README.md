# stuck-pod-reaper

A purpose-built Go Kubernetes utility for finding old, controller-owned Pending pods whose init or application containers remain in `ContainerCreating` or `PodInitializing`. I use it as a scheduled cleanup guard for pods that will be recreated by a Deployment, StatefulSet, Job, or other controller.

It is dry-run by default: matching pods are logged but not deleted.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--threshold` | `15m` | Minimum pod age |
| `--delete` | `false` | Actually delete matches |
| `--namespace` | all namespaces | Restrict the scan |
| `--kubeconfig` | in-cluster/default config | Explicit kubeconfig path |

```sh
docker run --rm -v "$HOME/.kube:/home/nonroot/.kube:ro" \
  davidcollom/stuck-pod-reaper:0.0.1 --threshold=30m
```

For dry-run, grant `list` on pods. When `--delete` is enabled, also grant `delete`; namespace-scoped RBAC is preferred. Standalone pods are ignored, as are pods already deleting or younger than the threshold. The program performs one scan and exits, making it suitable for a CronJob.
