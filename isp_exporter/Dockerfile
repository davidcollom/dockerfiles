FROM python:3.6-alpine

RUN mkdir -p               /opt/isp_exporter
ADD isp_exporter.py  /opt/isp_exporter
ADD requirements.txt       /opt/isp_exporter

RUN \
     pip3 install -r /opt/isp_exporter/requirements.txt

USER       nobody
EXPOSE     9353/tcp
WORKDIR    /opt/isp_exporter
ENTRYPOINT [ "python3", "isp_exporter.py" ]
