FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS build
ARG TARGETOS=linux
ARG TARGETARCH
ARG TARGETVARIANT
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -trimpath -ldflags="-s -w" -o /out/rpi-nfd-discovery .

FROM scratch
COPY --from=build /out/rpi-nfd-discovery /rpi-nfd-discovery
USER 65532:65532
ENTRYPOINT ["/rpi-nfd-discovery"]
