# ansible

An opinionated Ansible execution image for my infrastructure automation. Alpine's Ansible package is extended with OpenSSH, Git, Make, Just, kubectl, DNS and password-hash Python modules, plus the `ansible.posix` collection. This avoids reinstalling the same tools in CI jobs and Kubernetes automation pods.

The entrypoint is `ansible-playbook`; mount playbooks and inventory into the container. `/root/.ansible` is declared as a volume for roles, collections, and cache data.

```sh
docker run --rm -v "$PWD:/work" -w /work \
  -v "$HOME/.ssh:/root/.ssh:ro" \
  davidcollom/ansible:latest site.yml -i inventory.yml
```

SSH keys and kubeconfigs are sensitive. Mount them read-only and avoid baking them into derived images.
