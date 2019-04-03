IMAGE_NAME := davidcollom/packt-sync
IMAGE_TAG  ?= dev

LOG_LEVEL ?= DEBUG
USERNAME  ?= test@example.com
PASSWORD  ?= asifasgja


build: clean
	docker build --pull -t $(IMAGE_NAME):$(IMAGE_TAG) .

test: build
	docker run --rm -ti \
	-e "USERNAME=$(USERNAME)" \
	-e "PASSWORD=$(PASSWORD)" \
	-v $(PWD):/home:ro \
	$(IMAGE_NAME):$(IMAGE_TAG)

clean:
	docker rmi -f $(IMAGE_NAME):$(IMAGE_TAG)

publish:
	docker push $(IMAGE_NAME):$(IMAGE_TAG)