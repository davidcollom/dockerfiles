FROM --platform=${BUILDPLATFORM} golang:1.26.8-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN go test ./... && go vet ./...
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    GOARM=${TARGETVARIANT#v} go build -trimpath -ldflags="-s -w" -o /three-drop-sync .

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && \
    addgroup -g 10001 archiver && adduser -D -H -u 10001 -G archiver archiver && \
    mkdir /library && chown 10001:10001 /library
COPY --from=builder /three-drop-sync /usr/local/bin/three-drop-sync
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/three-drop-sync"]
