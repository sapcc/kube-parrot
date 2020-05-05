DATE    = $(shell date +%Y%m%d%H%M)
IMAGE   ?= sapcc/kube-plucked-parrot
VERSION = v$(DATE)
GOOS    ?= darwin

LDFLAGS := -X github.com/sapcc/kube-parrot/pkg/parrot.VERSION=$(VERSION)
GOFLAGS := -ldflags "$(LDFLAGS)"

CMDDIR   := cmd
PKGDIR   := pkg
PACKAGES := $(shell find $(CMDDIR) $(PKGDIR) -type d)
GOFILES  := $(addsuffix /*.go,$(PACKAGES))
GOFILES  := $(wildcard $(GOFILES))

.PHONY: all clean

all: bin/$(GOOS)/parrot

bin/%/parrot: $(GOFILES) Makefile
	GOOS=$* GOARCH=amd64 go build $(GOFLAGS) -v -i -o bin/$*/parrot ./cmd/parrot

build: bin/linux/parrot
	docker build -t $(IMAGE):$(VERSION) .

push:
	docker push $(IMAGE):$(VERSION)

clean:
	rm -rf bin/*
