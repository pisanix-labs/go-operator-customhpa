# Variáveis
IMG ?= ghcr.io/pisanix-labs/customhpa-controller:latest
PKG := github.com/pisanix-labs/go-operator-customhpa

.PHONY: all build run docker-build docker-push deploy undeploy crd rbac manager

all: build

build:
	GOOS=linux GOARCH=amd64 go build -o bin/manager ./cmd/manager

run:
	go run ./cmd/manager

docker-build:
	docker build -t $(IMG) .

docker-push:
	docker push $(IMG)

crd:
	kubectl apply -f config/crd/bases/monitoring.pisanix.dev_customhpas.yaml

rbac:
	kubectl apply -f config/rbac/service_account.yaml
	kubectl apply -f config/rbac/cluster_role.yaml
	kubectl apply -f config/rbac/cluster_role_binding.yaml

manager:
	kubectl apply -f config/manager/deployment.yaml
	# atualiza imagem do deployment
	kubectl -n default set image deployment/customhpa-controller manager=$(IMG)

deploy: crd rbac manager

undeploy:
	- kubectl delete -f config/manager/deployment.yaml --ignore-not-found --kubeconfig=./ops/kind/kubeconfig-kind.yaml
	- kubectl delete -f config/rbac/cluster_role_binding.yaml --ignore-not-found --kubeconfig=./ops/kind/kubeconfig-kind.yaml
	- kubectl delete -f config/rbac/cluster_role.yaml --ignore-not-found --kubeconfig=./ops/kind/kubeconfig-kind.yaml
	- kubectl delete -f config/rbac/service_account.yaml --ignore-not-found --kubeconfig=./ops/kind/kubeconfig-kind.yaml
	- kubectl delete -f config/crd/bases/monitoring.pisanix.dev_customhpas.yaml --ignore-not-found --kubeconfig=./ops/kind/kubeconfig-kind.yaml

