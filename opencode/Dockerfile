ARG UPSTREAM_VERSION=2.2.2
FROM ghcr.io/anomalyco/opencode:${UPSTREAM_VERSION}
ARG VERSION
LABEL org.opencontainers.image.title="OpenCode coding workspace" \
      org.opencontainers.image.source="https://github.com/davidcollom/dockerfiles" \
      org.opencontainers.image.version="${VERSION}"
USER root
RUN apk add --no-cache bash ca-certificates curl docker-cli docker-cli-buildx git github-cli jq nodejs npm openssh-client pipx python3 ripgrep uv \
    && addgroup -g 1000 opencode \
    && adduser -D -u 1000 -G opencode -h /home/opencode opencode \
    && mkdir -p /workspace /etc/opencode/command /home/opencode/.local/bin \
    && chown -R opencode:opencode /workspace /home/opencode
# Global OpenCode commands live in the system config directory and the checkout
# helper they call lives on PATH.
COPY command/ /etc/opencode/command/
COPY bin/opencode-checkout /usr/local/bin/opencode-checkout
RUN chmod 0755 /usr/local/bin/opencode-checkout
ENV HOME=/home/opencode \
    PATH="/home/opencode/.local/bin:${PATH}" \
    PIPX_HOME=/home/opencode/.local/pipx \
    PIPX_BIN_DIR=/home/opencode/.local/bin
WORKDIR /workspace
USER 1000:1000
EXPOSE 4096
ENTRYPOINT ["opencode"]
CMD ["serve", "--hostname", "0.0.0.0", "--port", "4096"]
