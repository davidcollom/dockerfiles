FROM ubuntu:18.04

ARG DEBIAN_FRONTEND=noninteractive

RUN \
    apt-get update && \
    apt-get install -y wget && \
    wget -q https://download.foldingathome.org/releases/public/release/fahclient/debian-stable-64bit/v7.5/latest.tar.bz2 -O /tmp/fah_latest.tar.bz2 && \
    tar xvf /tmp/fah_latest.tar.bz2 --strip 1 -C /usr/local/bin

EXPOSE 7396

ENTRYPOINT ["/usr/local/bin/FAHClient", "--web-allow=0/0:7396", "--allow=0/0:7396"]
