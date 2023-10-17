FROM python:3.12-slim

ARG VERSION

# Try and fix ENV issues in some of the Base images
# ENV PATH=/usr/local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

RUN apt-get update && \
    apt-get install -y nodejs curl python3-dev build-essential vim sqlite3 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

RUN pip install setuptools && \
    pip install flexget==$VERSION
RUN pip install "transmission-rpc>=4.1.4,<5.0.0" cfscrape cloudscraper pysocks
RUN ln -s /usr/bin/nodejs /usr/bin/node || true

WORKDIR /root/
ENV TZ Europe/London
CMD ["flexget -c /root/flexget.yml execute"]
