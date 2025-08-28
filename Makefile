APP_NAME := quota-manager
IMAGE_TAG ?= 1.0.0
REGISTRY ?= crpi-j8wrd0nl8l9v42wd.cn-shenzhen.personal.cr.aliyuncs.com/shenma-ai
IMAGE := $(REGISTRY)/$(APP_NAME):$(IMAGE_TAG)

.PHONY: docker-build
docker-build:
	docker build -t $(IMAGE) .

.PHONY: build-push
build-push: docker-build
	docker push $(IMAGE)

.PHONY: build-push-dockerhub
build-push-dockerhub: docker-build
	cd ~/sangfor/upload-docker-images/images-zgsm/ && \
	rm -f * && \
	docker save -o $(APP_NAME)-$(IMAGE_TAG).tar $(IMAGE) && \
	git add -A && \
	git commit -m "feat: add $(APP_NAME)-$(IMAGE_TAG).tar" && \
	git push origin main


