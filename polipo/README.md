# polipo

This image builds the discontinued Polipo caching web proxy from its source tag and copies only the binary and sample configuration into Alpine. It is retained for a legacy homelab use case where a small multi-architecture HTTP proxy is useful.

The default command is `polipo`; mount a reviewed configuration at `/etc/polipo/config` or supply command-line options.

Because Polipo is no longer actively maintained, do not expose it to untrusted networks. Prefer a maintained proxy for new deployments and restrict listening/bypass ACLs carefully.
