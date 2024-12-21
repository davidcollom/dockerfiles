#syntax=docker/dockerfile:1
FROM  --platform=${TARGETPLATFORM} python:3.12.3-bookworm

ARG VERSION
RUN apt-get update && \
    apt-get install -y nodejs curl python3-dev build-essential vim sqlite3 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Install Deps for arm32 - Run on all arch's doesn't make any differencegit
RUN curl -Lo /tmp/pendulum-3.0.0-py3-none-any.whl https://github.com/Flexget/Flexget/files/13787215/pendulum-3.0.0-py3-none-any.whl.zip && \
    pip install /tmp/pendulum-3.0.0-py3-none-any.whl

RUN pip install setuptools && \
    pip install flexget==$VERSION
RUN pip install "transmission-rpc>=7.0.3,<8.0.0" cfscrape cloudscraper pysocks
RUN ln -s /usr/bin/nodejs /usr/bin/node || true

WORKDIR /root/
ENV TZ Europe/London
CMD ["flexget -c /root/flexget.yml execute"]
