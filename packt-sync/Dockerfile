ARG VERSION
FROM alpine:latest

RUN apk add --no-cache --update perl perl-libwww perl-io-socket-ssl perl-uri perl-json perl-pathtools perl-log-log4perl make \
    && \
    rm -rf /var/cache/apk/*

# RUN cpanm -in URI::Escape LWP LWP::Protocol::https JSON Cwd Log::Log4perl

COPY packt-sync.pl /packt-sync.pl

ENV EXTENSIONS=epub,mobi,pdf

ENTRYPOINT ["/packt-sync.pl"]
