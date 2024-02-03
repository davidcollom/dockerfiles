FROM alpine:3.19.0

RUN apk add --no-cache \
	ca-certificates \
	openssh \
	git \
	ansible \
	make \
	just \
	py3-dnspython \
	py3-passlib \
  kubectl

RUN ansible-galaxy collection install ansible.posix

VOLUME /root/.ansible

ENTRYPOINT [ "ansible-playbook" ]
