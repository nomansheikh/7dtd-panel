# Paths in api/codegen.yaml are relative to the repository root, so every
# target here must run from the root.
.DEFAULT_GOAL := help
.PHONY: help generate bundle test vet fmt check build clean

SPEC_SRC := api/snapshot
BUNDLED  := api/openapi.bundled.yaml

help: ## Show this help.
	@awk 'BEGIN{FS=":.*##"; printf "Targets:\n"} /^[a-zA-Z_-]+:.*##/{printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

bundle: ## Rebuild the bundled OpenAPI spec from the vendored snapshot.
	go run ./tools/specbundle -from $(SPEC_SRC) -out api

generate: bundle ## Rebuild the spec and regenerate the typed API client.
	go tool oapi-codegen -config api/codegen.yaml $(BUNDLED)

refresh-spec: ## Re-fetch the spec snapshot from a live server, then rebuild.
	@test -n "$(SDTD_HOST)" || { echo "SDTD_HOST is required"; exit 1; }
	go run ./tools/specbundle -from http://$(SDTD_HOST):$(or $(SDTD_API_PORT),8080) -out api
	go tool oapi-codegen -config api/codegen.yaml $(BUNDLED)

vet: ## Run go vet.
	go vet ./...

test: ## Run the Go test suite.
	go test ./...

fmt: ## Format Go sources.
	gofmt -w $$(git ls-files '*.go' | grep -v '\.gen\.go$$')

check: vet test ## Everything CI gates on for the backend.

build: ## Build the panel binary.
	CGO_ENABLED=0 go build -trimpath -o 7dtd-panel ./cmd/7dtd-panel

clean: ## Remove build output.
	rm -f 7dtd-panel specbundle
