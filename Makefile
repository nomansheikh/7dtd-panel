# Paths in api/codegen.yaml are relative to the repository root, so every
# target here must run from the root.
.DEFAULT_GOAL := help
.PHONY: help generate bundle refresh-spec test vet fmt check build run clean frontend

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

frontend: ## Build the UI into internal/web/dist so the binary can embed it.
	rm -rf internal/web/dist/assets
	rm -f internal/web/dist/index.html internal/web/dist/favicon.svg
	cd web && npm ci && npx vp build

ui-add: ## Add shadcn components, e.g. make ui-add ITEMS="table command".
	@test -n "$(ITEMS)" || { echo 'ITEMS is required, e.g. make ui-add ITEMS="table dialog"'; exit 1; }
	cd web && npx shadcn@4.21.0 add $(ITEMS) --yes
	# shadcn emits `import { cn } from "cn"`, pulling in an npm package for a
	# three-line helper the project already has at @/lib/utils. Rewrite it and
	# drop the dependency, or it reappears on every add.
	cd web && sed -i '' 's|import { cn } from "cn"|import { cn } from "@/lib/utils"|' src/components/ui/*.tsx
	cd web && npm uninstall cn >/dev/null 2>&1 || true
	cd web && npx vp check --fix

frontend-check: ## Format, lint and type-check the UI.
	cd web && npx vp check

build: frontend ## Build the panel binary with the UI embedded.
	CGO_ENABLED=0 go build -trimpath -o 7dtd-panel ./cmd/7dtd-panel

build-api-only: ## Build the binary without rebuilding the UI.
	CGO_ENABLED=0 go build -trimpath -o 7dtd-panel ./cmd/7dtd-panel

run: ## Run the panel from source, reading configuration from .env.
	@test -f .env || { echo "no .env; copy .env.example and fill it in"; exit 1; }
	set -a && . ./.env && set +a && go run ./cmd/7dtd-panel

integration: ## Run tests that need a live game server. Requires SDTD_HOST.
	@test -n "$(SDTD_HOST)" || { echo "SDTD_HOST is required"; exit 1; }
	go test ./internal/sdtd -run Integration -v -count=1

clean: ## Remove build output.
	rm -f 7dtd-panel specbundle
	rm -rf internal/web/dist/assets internal/web/dist/index.html
