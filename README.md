# Efoy

Shared-commute platform for students and civil servants in Addis Ababa. The
design lives in [docs/](docs/): the software design document and the 30-day
implementation plan.

| Path | What |
| --- | --- |
| `cmd/` | Five Go binaries: `core-api`, `tracking-ingest`, `realtime-gateway`, `dispatch-engine`, `workers` |
| `internal/` | Business domains (`iam`, `trip`, `boarding`...) and platform wiring |
| `pkg/` | Shared, domain-free libraries |
| `api/openapi/efoy.yaml` | REST contract, the source of truth for server and client code |
| `db/migrations/` | goose SQL migrations (`0001_init.sql` is the full v1 schema) |
| `db/queries/` | SQL consumed by sqlc |
| `web/` | Next.js ops console and portals |

## Requirements

Go 1.26+, Node 20.9+ (22 recommended, see `web/.nvmrc`), Docker, and
[golangci-lint](https://golangci-lint.run) v2 for `make lint`.

## First-time setup

```sh
cp .env.example .env
make bootstrap        # Go tools, generated code, go.sum, web/node_modules
git add go.mod go.sum internal/api/apigen internal/db web/package-lock.json
```

Commit the files `make bootstrap` produces: CI needs `go.sum` and
`web/package-lock.json`, and checks that generated code is up to date.

## Run locally

```sh
docker compose up -d  # Postgres+PostGIS+TimescaleDB, Redis, NATS, MinIO, OSRM
make dev              # migrates, then runs all services with hot reload + the web console
```

- Console: http://localhost:3000
- core-api: http://localhost:8080/healthz, http://localhost:8080/readyz
- Other services: `:8081` tracking-ingest, `:8082` realtime-gateway, `:8083` dispatch-engine, `:8084` workers
- MinIO console: http://localhost:9001 (efoy / efoy-dev-secret)
- OSRM: http://localhost:5000. The first `docker compose up` downloads and preprocesses the Ethiopia OSM extract, which takes a few minutes.

`make help` lists every target. Useful ones: `make test`, `make lint`,
`make generate`, `make migrate-new name=...`, `make mock` (mock API from the
OpenAPI spec on :4010).


