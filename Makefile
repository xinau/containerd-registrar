MODULE   = $(shell go list -m)
VERSION  = $(shell git describe --tags --abbrev=0 --match=v* 2> /dev/null || echo "latest")
REVISION = $(shell git rev-parse HEAD)
LDFLAGS  = "-X $(MODULE)/internal/version.Version=$(VERSION) -X $(MODULE)/internal/version.Revision=$(REVISION)"
IMAGE    = xinau/containerd-registrar

.PHONY: build
build:
	docker build . \
		--tag $(IMAGE):$(VERSION) \
		--tag $(IMAGE):latest \
		--build-arg LDFLAGS=$(LDFLAGS)

.PHONY: publish
publish: build
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest

.PHONY: deploy
deploy: publish
	cat manifests/* \
	| sed -e 's|image: $(IMAGE):latest|image: $(IMAGE):$(VERSION)|g' \
	| sed -e 's|app.kubernetes.io/version: latest|app.kubernetes.io/version: $(VERSION)|g' \
	| kubectl apply -f -

