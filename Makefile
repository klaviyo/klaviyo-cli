# Dev tools are pinned and self-installed into ./bin so local runs and CI
# use identical versions (only Go itself is required). Bump the version here;
# CI runs the same targets.
# Note: keep compatible with the Go version in go.mod (CI sets
# GOTOOLCHAIN=local, so it cannot auto-download a newer toolchain).
GOLANGCI_LINT_VERSION := v2.12.2
GOLANGCI_LINT := bin/golangci-lint-$(GOLANGCI_LINT_VERSION)

.PHONY: build generate test lint fmt clean

build:
	go build -o bin/klaviyo ./cmd/klaviyo

generate:
	go generate ./...

test:
	go test -race ./...

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt

$(GOLANGCI_LINT):
	GOBIN=$(CURDIR)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	mv bin/golangci-lint $(GOLANGCI_LINT)

clean:
	rm -rf bin dist
