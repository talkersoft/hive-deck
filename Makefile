BINARY     := hv
PKG        := ./cmd/$(BINARY)
BIN        := bin/$(BINARY)
SYSTEM_BIN := /usr/local/bin/$(BINARY)
CONFIG_SRC := .hive
CONFIG_DST := $(HOME)/.hive

.PHONY: all build install install-config install-system uninstall-system setup run test fmt vet lint tidy clean help

all: build

## build: compile the CLI to ./bin/hv
build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -o $(BIN) $(PKG)

## install-config: mirror this repo's .hive/ to $HOME/.hive/
install-config:
	@mkdir -p $(CONFIG_DST)
	@cp -R $(CONFIG_SRC)/. $(CONFIG_DST)/
	@echo "configs synced to $(CONFIG_DST)/"

## install: user-level install — binary to $GOBIN (default ~/go/bin; ensure it's on $PATH) and configs to $HOME/.hive/
install: build install-config
	@cp $(BIN) $(shell go env GOPATH)/bin/$(BINARY)
	@echo "hv installed (run 'hv deck decks' to verify)"

## install-system: system-wide install — binary to /usr/local/bin (sudo) and configs to $HOME/.hive/ (no sudo for the configs)
install-system: build install-config
	sudo install -m 0755 $(BIN) $(SYSTEM_BIN)
	@echo "hv installed to $(SYSTEM_BIN)"

## uninstall-system: remove the CLI from /usr/local/bin (requires sudo). Does NOT touch $HOME/.hive/.
uninstall-system:
	sudo rm -f $(SYSTEM_BIN)

## setup: scaffold .hive/setup.yaml from the committed example (never overwrites; edit it to taste afterward)
setup:
	@mkdir -p $(CONFIG_SRC)
	@if [ ! -f $(CONFIG_SRC)/setup.yaml ]; then cp $(CONFIG_SRC)/setup.yaml.example $(CONFIG_SRC)/setup.yaml && echo "created $(CONFIG_SRC)/setup.yaml (edit it to taste)"; else echo "$(CONFIG_SRC)/setup.yaml exists, leaving alone"; fi

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
