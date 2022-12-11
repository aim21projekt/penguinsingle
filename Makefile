CGO_ENABLED=0
GOOS=linux
GOARCH=amd64
TAG=1.0.0
REPO=bashofmann/rancher-demo
GO111MODULE=on

GIT_TREE_STATE=$(shell (git status --porcelain | grep -q .) && echo dirty || echo clean)

all: build

test:
	@go test -v .

binary:
	@go build -ldflags '-w -linkmode external -extldflags -static' -o rancher-demo .

build:
	@docker build -t ${REPO}:${TAG} .

release:
ifeq ($(GIT_TREE_STATE),dirty)
	$(error git state is not clean)
endif
	git tag -a $(TAG) -m "Release $(TAG)"
	git push --tags

.PHONY: build binary release
