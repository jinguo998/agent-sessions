BINARY  := claude-sessions
PKG     := ./cmd/claude-sessions
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
.PHONY: build test test-race cover fmt fmt-check lint arch check clean install

build:
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o $(BINARY) $(PKG)

test:
	go test ./...

test-race:
	CGO_ENABLED=1 go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1

fmt:
	gofmt -w .

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

lint:
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	fi

arch:
	go test ./internal/architecture

check: fmt-check lint arch test

clean:
	rm -f $(BINARY) coverage.out
	rm -rf dist

install:
	go install -trimpath -ldflags "-X main.version=$(VERSION)" $(PKG)
