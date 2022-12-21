
NAMESPACE ?= davidcollom

DOCKER_USER 	  ?=
DOCKER_PASSWORD ?=
DOCKER_CONFIG   ?= $(HOME)/.docker/

login: setup
	echo "$(DOCKER_PASSWORD)" | docker login -u $(DOCKER_USER) --password-stdin

setup:
	# mkdir $(DOCKER_CONFIG)
	# cp docker.cfg $(DOCKER_CONFIG)/config.json

build: setup
	./build.sh $(NAMESPACE)