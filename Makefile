BINARY := kairos
BUILD_DIR := .
GO := go

.PHONY: build test vet lint clean install

build:
	$(GO) build -o $(BUILD_DIR)/$(BINARY) ./cmd/kairos

test:
	$(GO) test ./... -count=1

test-race:
	$(GO) test ./... -count=1 -race

vet:
	$(GO) vet ./...

lint: vet
	@echo "Lint passed (go vet)"

clean:
	rm -f $(BUILD_DIR)/$(BINARY)

install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || \
		cp $(BUILD_DIR)/$(BINARY) $(HOME)/go/bin/$(BINARY)

all: vet test build
