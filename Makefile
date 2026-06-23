.PHONY: all build clean fmt vet test tidy deps help

all: fmt vet test

build:
	@echo "Checking compilation..."
	@go build ./...

clean:
	@echo "Cleaning Go test cache..."
	@go clean -testcache
	@echo "Clean complete."

fmt:
	@echo "Formatting Go code..."
	@go fmt ./...

vet:
	@echo "Running go vet..."
	@go vet ./...

test:
	@echo "Running tests..."
	@go test -v ./...

tidy:
	@echo "Tidying go modules..."
	@go mod tidy

deps:
	@echo "Downloading dependencies..."
	@go mod download

help:
	@echo "Available targets:"
	@echo "  make build  - Verify package compilation"
	@echo "  make clean  - Clean package test cache"
	@echo "  make fmt    - Format Go code"
	@echo "  make vet    - Run go vet"
	@echo "  make test   - Run tests"
	@echo "  make tidy   - Tidy modules"
	@echo "  make deps   - Download modules"
