
FROM alpine:latest AS BUILDER
RUN apk add --update --virtual build-deps go git musl-dev libc-dev binutils make
ENV GOPATH=/go

ARG VERSION=master

RUN git clone https://github.com/mdlayher/unifi_exporter /app && \
    cd /app && \
    go mod init mdlayher/unifi_exporter && \
    go mod tidy && \
    go build -mod=mod -o /app/unifi_exporter ./cmd/unifi_exporter

FROM alpine:latest
COPY --from=BUILDER /app/unifi_exporter /bin/
EXPOSE 9130
USER nobody
ENTRYPOINT ["/bin/unifi_exporter"]
