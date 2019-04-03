FROM perl:5

RUN cpanm -in URI::Escape LWP LWP::Protocol::https JSON Cwd Log::Log4perl

COPY packt-sync.pl /packt-sync.pl

ENV EXTENSIONS=epub,mobi,pdf

ENTRYPOINT ["/packt-sync.pl"]