.PHONY: build test lint fmt

build:
	go build -o bin/klaviyo ./cmd/klaviyo

test:
	go test -race ./...

lint:
	golangci-lint run

fmt:
	golangci-lint fmt
