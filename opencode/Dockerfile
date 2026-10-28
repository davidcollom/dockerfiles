ARG UPSTREAM_VERSION=2.1.4
FROM ghcr.io/anomalyco/opencode:${UPSTREAM_VERSION}
ARG VERSION
LABEL org.opencontainers.image.title="OpenCode coding workspace" \
      org.opencontainers.image.source="https://github.com/davidcollom/dockerfiles" \
      org.opencontainers.image.version="${VERSION}"
USER root
RUN apk add --no-cache bash ca-certificates curl docker-cli docker-cli-buildx git github-cli jq nodejs npm openssh-client python3 ripgrep \
    && addgroup -g 1000 opencode \
    && adduser -D -u 1000 -G opencode -h /home/opencode opencode \
    && mkdir -p /workspace /etc/opencode \
    && chown opencode:opencode /workspace /home/opencode
ENV HOME=/home/opencode
WORKDIR /workspace
USER 1000:1000
EXPOSE 4096
ENTRYPOINT ["opencode"]
CMD ["serve", "--hostname", "0.0.0.0", "--port", "4096"]
