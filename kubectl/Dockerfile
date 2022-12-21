

FROM debian:latest
ARG VERSION

RUN apt-get update && \
    apt-get install -y ca-certificates curl jq \
    && apt-get clean && rm -rf /var/lib/apt/lists /var/cache/apt/archives

RUN curl -LO /usr/local/bin/kubectl https://dl.k8s.io/release/v${VERSION}/bin/linux/$(uname -m)/kubectl

USER 1001

ENTRYPOINT ["kubectl"]
CMD ["--help"]
