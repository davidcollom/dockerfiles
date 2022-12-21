FROM python:3.9-slim

ARG VERSION

RUN apt-get update && \
    apt-get install -y nodejs curl python3-dev build-essential vim sqlite3 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

RUN pip install setuptools && \
    pip install flexget==$VERSION
RUN pip install transmission-rpc cfscrape cloudscraper pysocks
RUN ln -s /usr/bin/nodejs /usr/bin/node || true

WORKDIR /root/
ENV TZ Europe/London
CMD ["flexget -c /root/flexget.yml execute"]
