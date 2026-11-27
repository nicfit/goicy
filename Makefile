SHELL := /usr/bin/env bash

include Makefile.golang

build:
	go build -o $(LOCALBIN)/goicy goicy.go
.PHONY: build

clean:
	rm -f $(LOCALBIN)/goicy
clean-dist:
	rm -rf $(LOCALBIN)/
.PHONY: clean clean-dist

test:
	go test ./...
.PHONY: test

all: clean build test
.PHONY: all

## Settings
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Linting

lint: golangci-lint  ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) version
	$(GOLANGCI_LINT) run
lint-fix: golangci-lint  ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix
.PHONY: lint lint-fix

GOLANGCI_LINT_VERSION ?= latest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))
