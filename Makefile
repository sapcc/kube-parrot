DATE    = $(shell date +%Y%m%d%H%M)
IMAGE   ?= keppel.eu-de-1.cloud.sap/ccloud/kube-parrot
VERSION = v$(DATE)
GOOS    ?= linux

LDFLAGS := -X github.com/sapcc/kube-parrot/pkg/parrot.VERSION=$(VERSION)
GOFLAGS := -ldflags "$(LDFLAGS)"

CMDDIR   := cmd
PKGDIR   := pkg
PACKAGES := $(shell find $(CMDDIR) $(PKGDIR) -type d)
GOFILES  := $(addsuffix /*.go,$(PACKAGES))
GOFILES  := $(wildcard $(GOFILES))
args = $(filter-out $@,$(MAKECMDGOALS))

export PARROT_IMAGE := $(IMAGE):$(VERSION)

.PHONY: all clean

all: bin/$(GOOS)/parrot

bin/%/parrot: $(GOFILES) Makefile
	GOOS=$* GOARCH=amd64 go build $(GOFLAGS) -v -i -o bin/$*/parrot ./cmd/parrot

build: bin/linux/parrot
	docker build -t $(PARROT_IMAGE) .

push:
	docker push $(PARROT_IMAGE)

clean:
	rm -rf bin/*

lab: build push start parrot

parrot:
	envsubst < ./testlab/parrot/kube-parrot.yaml | kubectl --kubeconfig kubeconfig.yaml --context default -n kube-system apply -f -

start:
	docker-compose up -d
	sleep 10

stop:
	docker-compose down

nodes:
	kubectl --kubeconfig kubeconfig.yaml --context default get node

pods:
	kubectl --kubeconfig kubeconfig.yaml --context default get pod -n kube-system

logs:
	kubectl --kubeconfig kubeconfig.yaml --context default -n kube-system logs $(call args)

sw1:
	docker exec -it arista-sw1 Cli

sw2:
	docker exec -it arista-sw2 Cli
