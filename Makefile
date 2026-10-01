SHELL := /bin/bash

# Local settings (copy .env.example to .env).
-include .env
export

MODULE   := github.com/blinge12/efoy
SERVICES := core-api tracking-ingest realtime-gateway dispatch-engine workers
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS  := -s -w -X $(MODULE)/internal/buildinfo.Version=$(VERSION) -X $(MODULE)/internal/buildinfo.Commit=$(COMMIT)

DATABASE_URL ?= $(EFOY_DATABASE__URL)
GOOSE        := go tool goose -dir db/migrations postgres "$(DATABASE_URL)"
SQLC         := docker run --rm -v "$(CURDIR)":/src -w /src/db sqlc/sqlc:1.29.0

# Local ports for `make dev` (every service listens on :8080 inside containers).
PORT_core-api         := 8080
PORT_tracking-ingest  := 8081
PORT_realtime-gateway := 8082
PORT_dispatch-engine  := 8083
PORT_workers          := 8084

.DEFAULT_GOAL := help
.PHONY: help bootstrap tidy generate generate-api generate-db check-generated \
        build test lint dev dev-web db-wait migrate-up migrate-down migrate-status \
        migrate-new compose-up compose-down mock docker-build staff gen-keys \
        staging-secrets helm-lint

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

bootstrap: ## One-time setup: Go tools, generated code, go.sum, web deps
	go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	go get -tool github.com/pressly/goose/v3/cmd/goose@latest
	go get -tool github.com/air-verse/air@latest
	$(MAKE) generate
	go mod tidy
	cd web && npm install

tidy: ## go mod tidy
	go mod tidy

generate: generate-api generate-db ## Regenerate all code from the OpenAPI spec and SQL

generate-api: ## Go server + models from api/openapi/efoy.yaml
	go tool oapi-codegen -config api/openapi/oapi-codegen.yaml api/openapi/efoy.yaml

generate-db: ## Type-safe queries from db/queries with sqlc (runs in Docker)
	$(SQLC) generate

check-generated: generate ## Fail if generated code is out of date
	git diff --exit-code -- internal/api/apigen internal/db

build: ## Build all service binaries into ./bin
	@mkdir -p bin
	@for s in $(SERVICES); do \
		echo "go build ./cmd/$$s"; \
		CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$$s ./cmd/$$s || exit 1; \
	done

test: ## Run Go tests
	go test -race -count=1 ./...

lint: ## Run golangci-lint (install: https://golangci-lint.run)
	golangci-lint run ./...

compose-up: ## Start Postgres/PostGIS/Timescale, Redis, NATS, MinIO and OSRM
	docker compose up -d

compose-down: ## Stop the local stack (data volumes are kept)
	docker compose down

db-wait:
	@echo "waiting for postgres..."
	@until docker compose exec -T postgres pg_isready -U efoy -d efoy >/dev/null 2>&1; do sleep 1; done

migrate-up: ## Apply all pending migrations
	$(GOOSE) up

migrate-down: ## Roll back the latest migration
	$(GOOSE) down

migrate-status: ## Show migration status
	$(GOOSE) status

migrate-new: ## Create a migration: make migrate-new name=add_foo
	go tool goose -dir db/migrations create $(name) sql

dev: db-wait migrate-up ## Run all services with hot reload plus the web console
	$(MAKE) -j $(words $(SERVICES) web) $(addprefix dev-,$(SERVICES)) dev-web

dev-web:
	cd web && npm run dev

dev-%:
	EFOY_HTTP__ADDR=:$(PORT_$*) go tool air \
		--tmp_dir "tmp/$*" \
		--build.cmd "go build -o ./tmp/$*/app ./cmd/$*" \
		--build.bin "./tmp/$*/app" \
		--build.exclude_dir "web,tmp,docs,deploy,bin"

mock: ## Serve mock responses from the OpenAPI spec on :4010 (Prism)
	docker run --rm -p 4010:4010 -v "$(CURDIR)/api/openapi":/spec stoplight/prism:5 mock -h 0.0.0.0 /spec/efoy.yaml

docker-build: ## Build Docker images for every service and the web console
	@for s in $(SERVICES); do \
		docker build --build-arg SERVICE=$$s --build-arg VERSION=$(VERSION) -t efoy-$$s:$(VERSION) . || exit 1; \
	done
	docker build -f web/Dockerfile -t efoy-web:$(VERSION) .

staff: ## Create a console account: make staff email=a@efoy.et name="Abebe Kebede" role=DISPATCHER
	go run ./cmd/core-api create-staff --email "$(email)" --name "$(name)" --role "$(or $(role),SUPER_ADMIN)"

gen-keys: ## Print a new JWT signing key and TOTP encryption key for an environment's secrets
	go run ./cmd/core-api gen-keys

staging-secrets: ## Decrypt deploy/secrets/staging.enc.yaml with SOPS and apply it to the current cluster
	sops --decrypt deploy/secrets/staging.enc.yaml | kubectl apply -f -

helm-lint: ## Lint and render the Helm charts (uses Docker)
	docker run --rm -v "$(CURDIR)":/apps -w /apps alpine/helm:3 lint deploy/helm/efoy -f deploy/helm/efoy/values-staging.yaml
	docker run --rm -v "$(CURDIR)":/apps -w /apps alpine/helm:3 lint deploy/helm/efoy-deps
