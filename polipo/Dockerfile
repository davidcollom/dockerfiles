FROM alpine:latest AS builder

ARG VERSION=1.1.1

RUN apk update && \
    apk add --no-cache build-base git texinfo && \
    git clone https://github.com/jech/polipo /tmp/polipo && \
    cd /tmp/polipo && \
    git checkout polipo-${VERSION} && \
    make all && \
    make install

FROM alpine:latest

COPY --from=builder /tmp/polipo/polipo /usr/local/sbin/polipo
COPY --from=builder /tmp/polipo/config.sample /etc/polipo/config


CMD ["polipo"]
