FROM python:3.9-alpine

RUN mkdir -p               /opt/speedtest_exporter
ADD speedtest_exporter.py  /opt/speedtest_exporter
ADD requirements.txt       /opt/speedtest_exporter

RUN pip3 install -r /opt/speedtest_exporter/requirements.txt

USER       nobody
EXPOSE     9353/tcp
WORKDIR    /opt/speedtest_exporter
ENTRYPOINT [ "python3", "speedtest_exporter.py" ]
