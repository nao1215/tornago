.PHONY: build test clean help tools changelog integration-test lint

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

build:  ## Build binary
	env GO111MODULE=on GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) $(GO_LDFLAGS) -o $(APP) cmd/$(APP)/main.go

clean: ## Clean project
	-rm -rf $(APP) coverage*

test: ## Run fast unit tests (excludes integration tests)
	env GOOS=$(GOOS) $(GO_TEST) -cover -coverpkg=$(GO_PKGROOT) -coverprofile=coverage.out -short $(GO_PKGROOT)
	-$(GO_TOOL) cover -html=coverage.out -o coverage.html

# go test's default timeout is 10 minutes, which was never a decision about this
# suite: driving a real Tor daemon takes 5 to 8 minutes when the network is well,
# and a slow descriptor or a slow bootstrap pushes it past the default and kills
# the run mid-test with a goroutine dump. The workflows that call this bound the
# job at 20 and 25 minutes, so the per-binary limit sits below the earlier of them.
integration-test: ## Run all tests including slow integration tests with full coverage
	$(INTEGRATION_ENV) TORNAGO_INTEGRATION=1 env GOOS=$(GOOS) $(GO_TEST) -timeout 15m -cover -coverprofile=coverage-integration.out -count=1 $(shell $(GO) list ./... | grep -v /examples)
	-$(GO_TOOL) cover -html=coverage-integration.out -o coverage-integration.html

lint: ## Run golangci-lint
	golangci-lint run

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
