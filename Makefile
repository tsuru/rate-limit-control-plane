BINARY=rate-limit-control-plane
IMAGE=localhost/rate-limit-control-plane-minikube:latest
TAG=latest
.PHONY: run
NAMESPACE=tsuru-system
SERVICE_ACCOUNT=rpaas-operator

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
