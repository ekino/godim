SHELL := $(shell which bash) # set default shell
GOFMT ?= gofmt "-s"
PACKAGES ?= $(shell go list ./... | grep -v /vendor/)
GOFILES := $(shell find . -name "*.go" -type f -not -path "./vendor/*")
# OS / Arch we will build our binaries for
OSARCH := "linux/amd64 linux/386 windows/amd64 windows/386 darwin/amd64 darwin/386"
ENV = /usr/bin/env

.SHELLFLAGS = -c # Run commands in a -c flag 
.SILENT: ; # no need for @
.ONESHELL: ; # recipes execute in same shell
.NOTPARALLEL: ; # wait for this target to finish
.EXPORT_ALL_VARIABLES: ; # send all vars to shell

.PHONY: all # All targets are accessible for user
.DEFAULT: help # Running Make will run the help target

help: ## Show Help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

dep: ## Get build dependencies
	go get -v -u github.com/golang/dep/cmd/dep; \
	go get github.com/mitchellh/gox; \
	go get github.com/mattn/goveralls; \
	go install golang.org/x/lint/golint;

lint: ## lint the code
	@hash golint > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u golang.org/x/lint/golint; \
	fi
	for PKG in $(PACKAGES); do golint -set_exit_status $$PKG || exit 1; done;

fmt-check: ## check for formatting issues
	@diff=$$($(GOFMT) -d $(GOFILES)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

build: ## Build the app
	dep ensure && go build

# cross-build: ## Build the app for multiple os/arch
#     gox -osarch=$(OSARCH) -output "bin/blackbeard_{{.OS}}_{{.Arch}}"

test: ## Launch tests
	go test -v ./…

test-cover: ## Launch tests coverage and send it to coverall
	$(ENV) ./scripts/test-coverage.sh
