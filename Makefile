BUILD_TIME=`date +%FT%T%z`
VERSION := $(shell sh -c 'git describe --always --tags')
BRANCH := $(shell sh -c 'git rev-parse --abbrev-ref HEAD')
COMMIT := $(shell sh -c 'git rev-parse --short HEAD')
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.branch=$(BRANCH) -X main.buildDate=$(BUILD_TIME)"
LINT_TOOL=$(shell go env GOPATH)/bin/golangci-lint
BUILD_TAGS=-tags go1.6
GO_PKGS=$(shell go list ./... | grep -v /vendor/ | grep -v /node_modules/)
GO_FILES=$(shell find . -type f -name '*.go' -not -path './vendor/*')

.PHONY: setup_dev build build-mac swagger fmt clean test lint qc deploy

setup: $(LINT_TOOL) setup_dev

setup_dev:
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/dep/cmd/dep	
	go get golang.org/x/tools/cmd/cover
	go get -u github.com/stripe/safesql

deps:
	dep ensure

build: deps
	env GOOS=linux GOARCH=amd64 go build $(BUILD_TAGS) $(LDFLAGS) -o bin/safesql safesql.go package16.go
	chmod +x bin/safesql

build-mac: deps
	env GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/safesql safesql.go package16.go
	chmod +x bin/safesql

fmt:
	@go fmt $(GO_PKGS)
	@goimports -w -l $(GO_FILES)

test:
	@go test -v $(shell go list ./... | grep -v /vendor/ | grep -v /node_modules/) -coverprofile=cover.out

clean:
	rm -rf ./bin ./vendor Gopkg.lock

$(LINT_TOOL):
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.16.0

qc: $(LINT_TOOL)
	$(LINT_TOOL) run --config=.golangci.yaml ./...

lint: qc

run:
	./bin/safesql
