FROM  --platform=${TARGETPLATFORM} alpine:3.11.3
EXPOSE 53 53/udp 953 8053

RUN apk --update add bind bash

RUN mkdir -m 0755 -p /var/run/named && chown -R root:named /var/run/named

# /var/cache/bind needs to be owned by "bind"
# since we are mounting, do it manually
# NOTE: Per Dockerfile manual --> need to mkdir the mounted dir to chown
RUN mkdir -m 0755 -p /var/cache/bind && touch /var/cache/bind/docker-init && chown -R named:named /var/cache/bind

COPY entrypoint.sh /
ENTRYPOINT ["/entrypoint.sh"]
