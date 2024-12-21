FROM --platform=${TARGETPLATFORM} alpine:3.19
ARG VERSION

RUN apk add --no-cache ddclient

ENTRYPOINT [ "ddclient" ]
