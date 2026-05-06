.PHONY: build fmt tidy vet lint

build:
	go build ./cmd/subtrans/

fmt:
	go fmt ./...

tidy:
	go mod tidy

vet:
	go vet ./...

lint:
	golangci-lint run ./...
