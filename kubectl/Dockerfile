FROM debian:stable-slim as builder
ARG VERSION

RUN apt-get update && \
    apt-get install -y ca-certificates curl jq \
    && apt-get clean && rm -rf /var/lib/apt/lists /var/cache/apt/archives

WORKDIR /
COPY install.sh /install.sh
RUN ./install.sh ${VERSION}


FROM debian:stable-slim

COPY --from=builder /kubectl /usr/local/bin/kubectl

USER 1001

ENTRYPOINT ["kubectl"]
CMD ["--help"]
