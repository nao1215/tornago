.PHONY: build test clean help tools changelog integration-test

APP         = tornago
VERSION     = $(shell git describe --tags --abbrev=0)
GIT_REVISION := $(shell git rev-parse HEAD)
GO          = go
GO_BUILD    = $(GO) build
GO_TEST     = $(GO) test -v
GO_TOOL     = $(GO) tool
GOOS        = ""
GOARCH      = ""
GO_PKGROOT  = ./...
GO_PACKAGES = $(shell $(GO_LIST) $(GO_PKGROOT))
GO_LDFLAGS  =

TOR_USE_EXTERNAL ?= 0
TOR_CONTROL      ?= 127.0.0.1:9051
TOR_SOCKS        ?= 127.0.0.1:9050
TOR_COOKIE       ?= $(HOME)/.tor/control.authcookie
TOR_PASSWORD     ?=

.PHONY: build test clean help tools changelog integration-test

build:  ## Build binary
	env GO111MODULE=on GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) $(GO_LDFLAGS) -o $(APP) cmd/$(APP)/main.go

clean: ## Clean project
	-rm -rf $(APP) coverage*

test: ## Run fast unit tests (excludes integration tests)
	env GOOS=$(GOOS) $(GO_TEST) -cover -coverpkg=$(GO_PKGROOT) -coverprofile=coverage.out -short $(GO_PKGROOT)
	$(GO_TOOL) cover -html=coverage.out -o coverage.html

integration-test: ## Run all tests including slow integration tests with full coverage
	$(INTEGRATION_ENV) TORNAGO_INTEGRATION=1 env GOOS=$(GOOS) $(GO_TEST) -cover -coverpkg=$(GO_PKGROOT) -coverprofile=coverage-integration.out -count=1 $(GO_PKGROOT)
	$(GO_TOOL) cover -html=coverage-integration.out -o coverage-integration.html

.DEFAULT_GOAL := help
help: ## Show help message
	@grep -E '^[0-9a-zA-Z_-]+[[:blank:]]*:.*?## .*$$' $(MAKEFILE_LIST) | sort \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[1;32m%-15s\033[0m %s\n", $$1, $$2}'

ifeq ($(TOR_USE_EXTERNAL),1)
ifeq ($(TOR_PASSWORD),)
INTEGRATION_ENV = TORNAGO_TOR_CONTROL=$(TOR_CONTROL) TORNAGO_TOR_SOCKS=$(TOR_SOCKS) TORNAGO_TOR_COOKIE=$(TOR_COOKIE)
else
INTEGRATION_ENV = TORNAGO_TOR_CONTROL=$(TOR_CONTROL) TORNAGO_TOR_SOCKS=$(TOR_SOCKS) TORNAGO_TOR_PASSWORD=$(TOR_PASSWORD)
endif
else
INTEGRATION_ENV =
endif
