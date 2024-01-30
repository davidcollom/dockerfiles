FROM python:3.12.1-bookworm

ENV ANSIBLE_VERSION=9.1.0

RUN curl https://sh.rustup.rs -sSf | sh -s -- -y
ENV PATH ~/.cargo/bin:$PATH

RUN pip3 install "ansible==${ANSIBLE_VERSION}"; \
    pip3 install ansible

VOLUME /root/.ansible

ENTRYPOINT [ "ansible" ]
