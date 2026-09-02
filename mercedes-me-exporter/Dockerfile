FROM --platform=${BUILDPLATFORM} golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOARCH)} go build -trimpath -ldflags="-s -w" -o /mercedes-me-exporter .

FROM alpine:3.23

RUN apk add --no-cache ca-certificates
COPY --from=builder /mercedes-me-exporter /bin/mercedes-me-exporter

USER nobody
EXPOSE 9353/tcp
ENTRYPOINT ["/bin/mercedes-me-exporter"]
