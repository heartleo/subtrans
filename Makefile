.PHONY: build lint

build:
	go build ./cmd/subtrans-cli/
	go build ./cmd/subtrans-server/

lint:
	golangci-lint run ./...
