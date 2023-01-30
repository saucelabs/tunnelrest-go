.PHONY: help doc install-dependencies install_tidied tidied untracked lint test clean fmt test coverage coverage_html ci

LOCALBIN ?= $(shell pwd)/bin
export GOBIN := $(shell pwd)/bin
export PATH := $(GOBIN):$(PATH)
.DEFAULT_GOAL := help

include .version

define PRINT_HELP_PYSCRIPT
import re, sys

for line in sys.stdin:
	match = re.match(r'^([0-9a-zA-Z_-]+):.*?## (.*)$$', line)
	if match:
		target, help = match.groups()
		print("%-20s %s" % (target, help))
endef

export PRINT_HELP_PYSCRIPT

help: ## Display this help screen
	@python -c "$$PRINT_HELP_PYSCRIPT" < $(MAKEFILE_LIST)

doc:
	@echo "Open http://localhost:6060/pkg/github.com/saucelabs/tunnelrest-go/ in your browser\n"
	@godoc -http :6060

install-dependencies: ## Install dev tool dependencies
	@rm -Rf bin && mkdir -p $(GOBIN)
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install golang.org/x/tools/cmd/godoc@latest
	go install gitlab.com/jamietanna/tidied@latest
	go install golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow@latest

tidied: ## Ensure `go mod tidy` has been run
	tidied -verbose

untracked: generate ## Check for no untracked files
	git status
	git diff-index --quiet HEAD --

lint: ## Lint the code
	golangci-lint run -v -c .golangci.yml && echo OK || (echo FAIL && exit 1)

test: ## Run tests
	go test -v -race -cover -coverprofile=coverage.out -run . ./...

coverage: test ## Generate coverage report
	go tool cover -func=coverage.out

coverage_html: test ## Generate HTML coverage report
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html

fmt: ## Format the code
	@golangci-lint run -c .golangci-fmt.yml --fix ./...
	@shadow -fix ./...

clean: ## Tidy up
	@rm -Rf coverage.* bin dist *.coverprofile *.dev *.race *.test *.log
	@go clean -cache -modcache -testcache ./... ||:

ci: fmt lint coverage ## fmt, lint and coverage
