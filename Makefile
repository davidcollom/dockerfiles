
NAMESPACE ?= davidcollom

DOCKER_USER 	?=
DOCKER_PASSWORD ?=

login: setup
	echo $(DOCKER_PASSWORD) | docker login -u $(DOCKER_USER) --password-stdin

setup:
	mkdir ./docker
	cp docker.cfg ./docker/config.json

build: setup
	./build.sh $(NAMESPACE)