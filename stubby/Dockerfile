FROM --platform=${TARGETPLATFORM} debian:trixie-slim AS builder
ARG VERSION
RUN apt-get update && \
    apt-get install -y  libev-dev libevent-dev libuv1-dev check \
                        build-essential cmake wget curl automake \
                        shtool libtool libidn11-dev python-dev-is-python3 libyaml-dev libssl-dev \
                        libtool-bin autoconf git make

RUN git clone https://github.com/getdnsapi/getdns.git

WORKDIR getdns

RUN git checkout v${VERSION} && git submodule update --init && mkdir build
WORKDIR build
RUN cmake -DBUILD_STUBBY=ON  -DUSE_LIBIDN2=OFF -DENABLE_STUB_ONLY=ON .. && make && make install && ldconfig


ARG VERSION
FROM --platform=${TARGETPLATFORM} debian:trixie-slim

RUN apt-get update && apt-get install -y libyaml-0-2 openssl ca-certificates && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=builder /usr/local/bin/stubby /stubby

EXPOSE 8053

CMD [ "/stubby" ]
