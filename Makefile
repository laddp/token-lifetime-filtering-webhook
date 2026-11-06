# Image to build/push
IMG ?= quay.io/example/token-lifetime-webhook:latest

# Get the currently used golang install path if not defined
GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

CONTROLLER_GEN ?= $(GOBIN)/controller-gen
CONTROLLER_TOOLS_VERSION ?= v0.16.5

.PHONY: all build test fmt vet docker-build deploy undeploy generate manifests clean

all: build

build:
	CGO_ENABLED=0 go build -o bin/webhook ./cmd/webhook

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

generate: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

manifests: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd

docker-build:
	docker build -t $(IMG) .

deploy:
	kubectl apply -f config/crd
	kubectl wait --for=condition=Established crd/tokenlifetimepolicies.tokenfiltering.openshift.io --timeout=120s
	kubectl apply -f config/deploy
	kubectl rollout status deployment/token-lifetime-webhook -n token-lifetime-webhook --timeout=180s
	kubectl apply -f config/samples/tokenlifetimepolicy-cluster.yaml
	kubectl apply -f config/webhook
	kubectl wait --for=jsonpath='{.webhooks[0].clientConfig.caBundle}' mutatingwebhookconfiguration/token-lifetime-webhook --timeout=120s

undeploy:
	kubectl delete -f config/webhook --ignore-not-found
	kubectl delete -f config/samples/tokenlifetimepolicy-cluster.yaml --ignore-not-found
	kubectl delete -f config/deploy --ignore-not-found
	kubectl delete -f config/crd --ignore-not-found

$(CONTROLLER_GEN):
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

clean:
	rm -rf bin/
