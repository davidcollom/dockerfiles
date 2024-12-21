FROM --platform=${TARGETPLATFORM} golang:1.14-buster as builder
ARG VERSION
RUN git clone https://github.com/jetstack/version-checker.git
WORKDIR /go/version-checker
RUN git checkout v${VERSION}

RUN mkdir -p ./bin/ && \
    CGO_ENABLED=0 go build -o ./bin/version-checker-linux ./cmd/.


# Actually build image.
FROM --platform=${TARGETPLATFORM} alpine:3.12

RUN apk --no-cache add ca-certificates

COPY --from=builder /go/version-checker/bin/version-checker-linux /usr/bin/version-checker

USER 65534

ENTRYPOINT ["/usr/bin/version-checker"]
