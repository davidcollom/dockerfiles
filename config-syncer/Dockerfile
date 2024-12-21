FROM --platform=${TARGETPLATFORM} python:3.9-slim

RUN mkdir -p               /opt/exporter
COPY requirements.txt       /opt/exporter

RUN \
     pip3 install -r /opt/exporter/requirements.txt

COPY config_syncer.py  /opt/exporter/

USER       nobody
EXPOSE     9353/tcp
WORKDIR    /opt/exporter/
ENTRYPOINT [ "python3", "config_syncer.py" ]
