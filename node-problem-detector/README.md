# node-problem-detector

This image rebuilds Kubernetes node-problem-detector with journald support and includes its health checker and configuration. I maintain it to produce compatible images for all architectures in my cluster and to ensure system-log monitoring works on systemd-based nodes.

The default entrypoint runs the kernel monitor configuration:

```text
/node-problem-detector --config.system-log-monitor=/config/kernel-monitor.json
```

Deploy it as a privileged or appropriately permissioned DaemonSet following upstream guidance. It needs access to host logs/system state and the Kubernetes API to report node conditions; those permissions are security-sensitive.
