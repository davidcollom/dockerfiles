FROM debian:sid

RUN apt-get update && \
    apt-get install -y polipo && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY config /etc/polipo/config
COPY options /etc/polipo/options

EXPOSE 8123

CMD ["polipo"]
