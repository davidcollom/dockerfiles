# tailscale

This image extends the official Tailscale image with startup logic tailored to Kubernetes. It stores daemon state in a Kubernetes Secret, supports userspace networking, can advertise routes, and can optionally DNAT the Tailscale address to another destination.

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `AUTH_KEY` | empty | Optional Tailscale auth key |
| `ROUTES` | empty | Routes to advertise |
| `DEST_IP` | empty | Optional DNAT destination |
| `EXTRA_ARGS` | empty | Extra arguments to `tailscale up` |
| `USERSPACE` | `true` | Use userspace networking |
| `KUBE_SECRET` | `tailscale` | Kubernetes Secret used for state |

Userspace mode does not support `DEST_IP`. Kernel mode needs `/dev/net/tun`, network capabilities, and iptables support. The pod's service account also needs access to the chosen state Secret. Auth keys should be injected as secrets and preferably be ephemeral/pre-authorized with limited tags.
