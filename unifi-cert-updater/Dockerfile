FROM --platform=${TARGETPLATFORM} golang:1.24-alpine AS builder
ARG VERSION

WORKDIR /go/unifi-cert-updater

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /unifi-cert-updater .

# Actually build image.
FROM --platform=${TARGETPLATFORM} alpine:3.19

RUN apk --no-cache add ca-certificates

COPY --from=builder /unifi-cert-updater /usr/bin/unifi-cert-updater

USER 65534

ENTRYPOINT ["/usr/bin/unifi-cert-updater"]
