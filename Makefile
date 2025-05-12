BINARY=rate-limit-control-plane
IMAGE=localhost/rate-limit-control-plane-minikube:latest
TAG=latest
.PHONY: run
NAMESPACE=tsuru-system
SERVICE_ACCOUNT=rpaas-operator

# Run tests
.PHONY: test
test: fmt vet lint
	go test -race -coverprofile cover.out ./...

.PHONY: lint
lint: golangci-lint
	$(GOLANGCI_LINT) run ./...

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# find or download golangci-lint
# download golangci-lint if necessary
.PHONY: golangci-lint
golangci-lint:
ifeq (, $(shell which golangci-lint))
	@{ \
	set -e ;\
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v2.1.6 -- -b $(GOBIN) ;\
	}
GOLANGCI_LINT=$(GOBIN)/golangci-lint
else
GOLANGCI_LINT=$(shell which golangci-lint)
endif


run:
	go run ./main.go --enable-leader-election=false

build:
	go build -o $(BINARY) ./main.go

.PHONY: build-docker-minikube
build-docker-minikube:
	docker build -t $(IMAGE) .

.PHONY: save-docker-minikube
save-docker-minikube: build-docker-minikube
	docker save $(IMAGE) | minikube image load -

.PHONY: minikube-run
minikube-run: save-docker-minikube
	kubectl run rate-limit-control-plane --rm -i --tty --image $(IMAGE) \
		-n $(NAMESPACE) --image-pull-policy Never \
		--overrides='{ "spec": { "serviceAccountName": "rpaas-operator" } }'
