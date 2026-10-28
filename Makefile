BINARY_NAME := rexamine
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_DIR := bin
INSTALL_PATH := $(HOME)/.local/bin
LDFLAGS := -s -w -X main.version=$(VERSION)
# Embeds only the tree-sitter grammars that rexamine uses, which keeps the
# binary small. Keep in sync with treeSitterLanguages and .goreleaser.yaml.
GRAMMAR_TAGS := grammar_subset,grammar_subset_bash,grammar_subset_c,grammar_subset_c_sharp,grammar_subset_cpp,grammar_subset_css,grammar_subset_dart,grammar_subset_dockerfile,grammar_subset_elixir,grammar_subset_go,grammar_subset_gomod,grammar_subset_graphql,grammar_subset_hcl,grammar_subset_java,grammar_subset_javascript,grammar_subset_json,grammar_subset_kotlin,grammar_subset_lua,grammar_subset_make,grammar_subset_proto,grammar_subset_python,grammar_subset_ruby,grammar_subset_rust,grammar_subset_scala,grammar_subset_sql,grammar_subset_swift,grammar_subset_toml,grammar_subset_tsx,grammar_subset_typescript,grammar_subset_yaml,grammar_subset_zig

# Tools run through `go run` at pinned versions, so no global install is needed.
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
GORELEASER := go run github.com/goreleaser/goreleaser/v2@v2.19.0

## build: Build the binary for this machine into bin/.
build:
	go build -tags "$(GRAMMAR_TAGS)" -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) .

## install: Install the binary to ~/.local/bin.
install: build
	mkdir -p $(INSTALL_PATH)
	# macOS kills a binary that is overwritten in place, so replace the file instead.
	rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)

## setup: Install the binary, then the Rex module and the agent skill.
setup: install
	$(INSTALL_PATH)/$(BINARY_NAME) setup

## test: Run the tests with the race detector.
test:
	go test -race ./...

## lint: Run go vet and golangci-lint.
lint:
	go vet ./...
	$(GOLANGCI_LINT) run ./...

## fmt: Format the code.
fmt:
	$(GOLANGCI_LINT) fmt ./...

## check: Run every check that CI runs.
check: lint test

## snapshot: Build release archives for every platform into dist/, without publishing.
snapshot:
	$(GORELEASER) release --snapshot --clean --skip=publish

## clean: Remove build output.
clean:
	rm -rf $(BUILD_DIR) dist

## version: Print the version that a build gets.
version:
	@echo $(VERSION)

## help: List the targets.
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

.PHONY: build install setup test lint fmt check snapshot clean version help
