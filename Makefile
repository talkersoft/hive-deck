BINARY     := hv
PKG        := ./cmd/$(BINARY)
BIN        := bin/$(BINARY)
SYSTEM_BIN := /usr/local/bin/$(BINARY)

.PHONY: all build install install-system uninstall-system run test fmt vet lint tidy clean help

all: build

## build: compile the CLI to ./bin/hv
build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -o $(BIN) $(PKG)

## install: user-level install — binary to $GOBIN (default ~/go/bin; ensure it's on $PATH)
install: build
	@cp $(BIN) $(shell go env GOPATH)/bin/$(BINARY)
	@echo "hv installed (run 'hv deck decks' to verify)"

## install-system: system-wide install — binary to /usr/local/bin (sudo)
install-system: build
	sudo install -m 0755 $(BIN) $(SYSTEM_BIN)
	@echo "hv installed to $(SYSTEM_BIN)"

## uninstall-system: remove the CLI from /usr/local/bin (requires sudo).
uninstall-system:
	sudo rm -f $(SYSTEM_BIN)

## run: build and run (pass ARGS="..." for cli args)
run: build
	$(BIN) $(ARGS)

## test: run all tests
test:
	go test ./...

## fmt: format every Go file
fmt:
	go fmt ./...

## vet: static analysis
vet:
	go vet ./...

## lint: fmt + vet + (golangci-lint if installed)
lint: fmt vet
	@command -v golangci-lint >/dev/null && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

## tidy: clean go.mod / go.sum
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -rf bin/

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
