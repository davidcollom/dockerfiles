FROM alpine:latest AS clone
ARG VERSION
RUN apk add --no-cache git

RUN git clone https://github.com/philiptn/mkv-auto /mkv-auto/
WORKDIR /mkv-auto
RUN git checkout ${VERSION:-master} || true && \
    git submodule update --init --recursive

FROM ubuntu:22.04

COPY --from=clone /mkv-auto /mkv-auto/
COPY config.ini /mkv-auto/
WORKDIR /mkv-auto
RUN ./prerequisites.sh

ENTRYPOINT ["/mkv-auto/entrypoint.sh"]
