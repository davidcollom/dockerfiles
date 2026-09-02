FROM --platform=${BUILDPLATFORM} golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOARCH)} go build -trimpath -ldflags="-s -w" -o /packt-sync .

FROM alpine:3.23

RUN apk add --no-cache ca-certificates
COPY --from=builder /packt-sync /bin/packt-sync

ENV EXTENSIONS=epub,mobi,pdf
ENV DOWNLOAD_CONCURRENCY=4

ENTRYPOINT ["/bin/packt-sync"]
