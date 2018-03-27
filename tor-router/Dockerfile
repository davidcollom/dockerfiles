FROM debian:sid

RUN apt-get update && \
    apt-get install -y tor --no-install-recommends && \
    rm -rf /var/lib/apt/lists/*

COPY torrc /etc/tor/torrc

RUN chown -Rfv debian-tor:debian-tor /etc/tor/torrc

USER debian-tor

ENTRYPOINT [ "tor" ]
CMD [ "-f", "/etc/tor/torrc.default" ]
