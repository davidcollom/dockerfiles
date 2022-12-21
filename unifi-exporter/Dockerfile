
FROM alpine AS BUILDER

RUN apk add --update --virtual build-deps go git musl-dev libc-dev binutils 

RUN GOOS=linux go get -a -ldflags '-extldflags "-static"' github.com/mdlayher/unifi_exporter/cmd/unifi_exporter && \
    mv ~/go/bin/unifi_exporter /bin/


FROM alpine
COPY --from=BUILDER /bin/unifi_exporter /bin/
EXPOSE 9130
USER nobody
ENTRYPOINT ["/bin/unifi_exporter"]
