FROM python:3.12.1-bookworm

ENV ANSIBLE_VERSION 2.9.17

RUN apt-get update; \
    apt-get install -y gcc python3; \
    apt-get install -y python3-pip; \
    apt-get clean all

RUN pip3 install "ansible==${ANSIBLE_VERSION}"; \
    pip3 install ansible
