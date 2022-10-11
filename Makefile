MODULE   = $(shell go list -m)
VERSION  = $(shell git describe --tags --match=v* 2> /dev/null)
REVISION = $(shell git rev-parse HEAD)
LDFLAGS  = "-X $(MODULE)/internal/version.Version=$(VERSION) -X $(MODULE)/internal/version.Revision=$(REVISION)"
IMAGE    = xinau/containerd-registrar

.PHONY: build
build:
	docker build . \
		--tag docker.io/$(IMAGE):$(VERSION) \
		--tag docker.io/$(IMAGE):latest \
		--tag ghcr.io/$(IMAGE):$(VERSION) \
		--tag ghcr.io/$(IMAGE):latest \
		--build-arg LDFLAGS=$(LDFLAGS)

.PHONY: publish
publish: build
	docker push docker.io/$(IMAGE):$(VERSION)
	docker push docker.io/$(IMAGE):latest
	docker push ghcr.io/$(IMAGE):$(VERSION)
	docker push ghcr.io/$(IMAGE):latest

.PHONY: deploy
deploy: publish
	cat manifests/* \
	| sed -e 's|image: $(IMAGE):latest|image: $(IMAGE):$(VERSION)|g' \
	| sed -e 's|app.kubernetes.io/version: latest|app.kubernetes.io/version: $(VERSION)|g' \
	| kubectl apply -f -

