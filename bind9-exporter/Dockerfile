FROM alpine:latest AS BUILDER

RUN apk add --update --virtual build-deps go git musl-dev libc-dev binutils make
ENV GOPATH=/go

ARG VERSION=0.6.0

RUN go install github.com/prometheus-community/bind_exporter@v${VERSION}

FROM alpine:latest
COPY --from=BUILDER /go/bin/bind_exporter /bin/
EXPOSE 9119
USER nobody
ENTRYPOINT ["/bin/bind_exporter"]
