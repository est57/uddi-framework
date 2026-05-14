# UDDI API Gateway

Go REST API gateway for local UDDI development.

## Current Status

This package is an alpha gateway with DID registry storage. It can run with an in-memory store for tests or a Postgres-backed store when `UDDI_DATABASE_URL` is configured.

Implemented today:

- Health endpoint.
- DID register, resolve, and revoke.
- API key middleware.
- API key validation with seeded development credentials.
- Admin API key management for creating, listing, and revoking service API keys.
- Auth challenge creation.
- Auth presentation verification with Ed25519.
- Challenge replay protection.
- Redis-backed challenge storage when `UDDI_REDIS_URL` is configured.
- Service binding and presentation timestamp validation.
- `DIDStore` abstraction with in-memory and Postgres implementations.
- Credential registry with in-memory and Postgres implementations.
- Ed25519 Verifiable Credential proof verification before storing and during status verification.
- ZKP endpoint stubs.

## Run Checks

```bash
go test ./...
go vet ./...
```

If your environment cannot write to the default Go cache:

```bash
mkdir -p .cache/go-build .cache/go-mod
GOCACHE="$PWD/.cache/go-build" GOMODCACHE="$PWD/.cache/go-mod" go test ./...
GOCACHE="$PWD/.cache/go-build" GOMODCACHE="$PWD/.cache/go-mod" go vet ./...
```

## Run Locally

```bash
go run .
```

Default address:

```text
:8080
```

Configuration is read from environment variables:

- `UDDI_HTTP_ADDR`
- `UDDI_GRPC_ADDR`
- `UDDI_BLOCKCHAIN_RPC`
- `UDDI_DATABASE_URL`
- `UDDI_REDIS_URL`
- `UDDI_ADMIN_TOKEN`
- `UDDI_ZKP_SERVICE_URL`
- `UDDI_ALLOWED_ORIGINS`

## Run With Docker

From the repository root:

```bash
docker build -t uddi-api ./packages/api
docker run --rm -p 8080:8080 uddi-api
```

Then check:

```bash
curl http://localhost:8080/health
```

## Run With Docker Compose

From the repository root:

```bash
docker compose -f infra/docker/docker-compose.dev.yml up --build
```

If Docker reports orphan containers from another local project, clean the old default compose project first:

```bash
docker compose -f infra/docker/docker-compose.dev.yml down --remove-orphans
```

If you previously ran an older compose file with fixed container names and see `container name is already in use`, remove those old containers:

```bash
docker rm uddi-api uddi-postgres uddi-redis
```

If Postgres logs `role "uddi" does not exist`, remove the old UDDI dev volume and start again:

```bash
docker compose -f infra/docker/docker-compose.dev.yml down -v --remove-orphans
docker compose -f infra/docker/docker-compose.dev.yml up --build
```

The current compose stack runs:

- API gateway
- Redis
- Postgres

Blockchain and ZKP runtime containers are intentionally not included yet because those services are still roadmap items in this repository.

With compose running, run the Postgres-backed DID store integration test from the repository root:

```bash
pnpm api:test:postgres
```

Run the Redis-backed challenge store integration test:

```bash
pnpm api:test:redis
```

## API Key

Protected routes require:

```text
X-API-Key: <key>
X-Service-ID: <service id>
```

The middleware validates both headers against the configured API key store.
For local development, the API key store seeds:

```text
X-Service-ID: dev-service
X-API-Key: dev-api-key
```

Tests also seed:

```text
X-Service-ID: test-service
X-API-Key: test-key
```
