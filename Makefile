BINARY := kairos
MCP_BINARY := kairos-mcp
BUILD_DIR := .
GO := go

.PHONY: build build-mcp test test-cover test-race vet lint clean install all

build:
	$(GO) build -o $(BUILD_DIR)/$(BINARY) ./cmd/kairos

build-mcp:
	$(GO) build -o $(BUILD_DIR)/$(MCP_BINARY) ./cmd/kairos-mcp

test:
	$(GO) test ./... -count=1

test-race:
	$(GO) test ./... -count=1 -race

test-cover:
	$(GO) test ./... -count=1 -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out

vet:
	$(GO) vet ./...

lint: vet
	@echo "Lint passed (go vet)"

clean:
	rm -f $(BUILD_DIR)/$(BINARY) $(BUILD_DIR)/$(MCP_BINARY)
	rm -f coverage.out

install: build build-mcp
	mkdir -p $(firstword $(GOPATH) $(HOME)/go)/bin
	cp $(BUILD_DIR)/$(BINARY) $(firstword $(GOPATH) $(HOME)/go)/bin/$(BINARY)
	cp $(BUILD_DIR)/$(MCP_BINARY) $(firstword $(GOPATH) $(HOME)/go)/bin/$(MCP_BINARY)

all: vet test build build-mcp
