FROM python:3.5-jessie

RUN apt-get update && \
    apt-get install -y nodejs && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    ln -s /usr/bin/nodejs /usr/bin/node

RUN pip install  --no-cache-dir --upgrade setuptools && \
    pip install  --no-cache-dir flexget transmissionrpc cfscrape

CMD ["flexget", "execute"]
