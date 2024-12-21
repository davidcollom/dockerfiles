FROM --platform=${TARGETPLATFORM} python:3.9-alpine

RUN mkdir -p               /opt/exporter
COPY requirements.txt       /opt/exporter

RUN \
     pip3 install -r /opt/exporter/requirements.txt

COPY mercedes_me.py  /opt/exporter/

USER       nobody
EXPOSE     9353/tcp
WORKDIR    /opt/exporter/
ENTRYPOINT [ "python3", "mercedes_me.py" ]
