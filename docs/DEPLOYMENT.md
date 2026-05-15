# UDDI Deployment Guide

This guide describes how to run the current UDDI alpha API in a shared or production-like environment.

UDDI is not a production identity network yet. The API, SDK, DID registry, credential registry, API key management, migrations, Redis challenge store, OpenAPI contract, and observability endpoints are implemented. The blockchain registry and ZKP prover/verifier are still adapter-backed roadmap areas.

## Runtime Components

Run these services:

- UDDI API gateway
- Postgres 16 or newer
- Redis 7 or newer
- Reverse proxy or load balancer with TLS termination

Current adapters:

- DID registry persistence: Postgres
- Credential registry persistence: Postgres
- API key store: Postgres
- Auth challenge store: Redis
- Blockchain registry: `memory://local` adapter
- ZKP service: `memory://zkp` or external HTTP adapter

## Required Environment

Set these for any shared environment:

```bash
UDDI_ENV=production
UDDI_HTTP_ADDR=:8080
UDDI_BLOCKCHAIN_RPC=memory://local
UDDI_DATABASE_URL=postgres://USER:PASSWORD@postgres:5432/uddi?sslmode=disable
UDDI_REDIS_URL=redis://redis:6379
UDDI_ZKP_SERVICE_URL=memory://zkp
UDDI_ALLOWED_ORIGINS=https://your-app.example
UDDI_MAX_REQUEST_BODY_BYTES=1048576
UDDI_RATE_LIMIT_REQUESTS=120
UDDI_RATE_LIMIT_WINDOW_SECONDS=60
UDDI_ADMIN_TOKEN=replace-with-a-long-random-secret
UDDI_JWT_SECRET=replace-with-a-long-random-secret
```

Generate secrets with at least 32 bytes of entropy:

```bash
openssl rand -base64 32
```

Do not use the development values from `.env.example` or `infra/docker/docker-compose.dev.yml` in a shared environment.

## Production-Like Compose

This is a reference shape. Store real secrets in a secret manager or protected environment file.

```yaml
name: uddi-production

services:
  api:
    build:
      context: ./packages/api
      dockerfile: Dockerfile
    # Or use your own published image:
    # image: ghcr.io/YOUR_ORG/uddi-api:TAG
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      UDDI_ENV: production
      UDDI_HTTP_ADDR: :8080
      UDDI_BLOCKCHAIN_RPC: memory://local
      UDDI_DATABASE_URL: postgres://uddi:${POSTGRES_PASSWORD}@postgres:5432/uddi?sslmode=disable
      UDDI_REDIS_URL: redis://redis:6379
      UDDI_ZKP_SERVICE_URL: memory://zkp
      UDDI_ALLOWED_ORIGINS: https://your-app.example
      UDDI_MAX_REQUEST_BODY_BYTES: 1048576
      UDDI_RATE_LIMIT_REQUESTS: 120
      UDDI_RATE_LIMIT_WINDOW_SECONDS: 60
      UDDI_ADMIN_TOKEN: ${UDDI_ADMIN_TOKEN}
      UDDI_JWT_SECRET: ${UDDI_JWT_SECRET}
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:8080/health >/dev/null"]
      interval: 10s
      timeout: 3s
      retries: 5

  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: uddi
      POSTGRES_USER: uddi
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U uddi -d uddi"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    command: redis-server --save 60 1 --loglevel warning
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  postgres_data:
  redis_data:
```

## Startup And Migrations

When `UDDI_DATABASE_URL` is set, the API runs versioned Postgres migrations on startup. Applied migrations are tracked in `schema_migrations`.

The API retries initial Postgres connection during startup. On Docker Desktop or slow orchestrators, logs like this can be normal while DNS and Postgres are coming up:

```text
DID database not ready; retrying
```

If the API still fails after retries, check:

- Postgres hostname in `UDDI_DATABASE_URL`
- Docker network or Kubernetes service DNS
- Postgres user, password, and database name
- Postgres health check status

## API Key Bootstrap

Create service API keys with the admin endpoint:

```bash
curl -X POST http://localhost:8080/v1/admin/api-keys/ \
  -H "Content-Type: application/json" \
  -H "X-Admin-Token: $UDDI_ADMIN_TOKEN" \
  -d '{
    "serviceId": "my-service",
    "serviceName": "My Service"
  }'
```

The response includes a plaintext `apiKey` once. Store it securely; the API stores only a hash.

Use the key on protected endpoints:

```http
X-Service-ID: my-service
X-API-Key: uddi_GENERATED_SECRET
```

Revoke a key:

```bash
curl -X POST http://localhost:8080/v1/admin/api-keys/revoke \
  -H "Content-Type: application/json" \
  -H "X-Admin-Token: $UDDI_ADMIN_TOKEN" \
  -d '{"serviceId": "my-service"}'
```

## Health And Operations

Use these endpoints:

- `GET /health` for liveness
- `GET /ready` for readiness
- `GET /metrics` for JSON runtime metrics
- `GET /openapi.yaml` for OpenAPI 3.1
- `GET /docs` for Swagger UI

Example:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/metrics
```

## Smoke Test

After starting the local Docker stack, run the E2E smoke test from the repository root:

```bash
pnpm docker:dev
pnpm e2e:smoke
```

The smoke test creates a temporary service API key through the admin endpoint, registers a holder DID, rotates its signing key, verifies an auth challenge, issues and revokes a credential, and checks `/health`, `/ready`, `/metrics`, `/openapi.yaml`, and `/docs`.

For a non-default endpoint:

```bash
UDDI_API_URL=https://your-api.example \
UDDI_ADMIN_TOKEN="$UDDI_ADMIN_TOKEN" \
pnpm e2e:smoke
```

## Backups

Back up Postgres regularly. At minimum:

```bash
pg_dump "$UDDI_DATABASE_URL" > uddi-backup.sql
```

For Docker Compose:

```bash
docker exec uddi-production-postgres-1 pg_dump -U uddi -d uddi > uddi-backup.sql
```

Redis currently stores short-lived auth challenges. Losing Redis invalidates in-flight challenges but does not lose DID or credential registry records.

## Reverse Proxy

Terminate TLS at a reverse proxy or load balancer. Recommended basics:

- Force HTTPS
- Set `UDDI_ALLOWED_ORIGINS` to exact trusted origins
- Keep `/v1/admin/*` restricted by network policy where possible
- Consider rate limiting at the edge as well as in the API
- Do not expose Postgres or Redis publicly

## Security Notes

- Rotate `UDDI_ADMIN_TOKEN` before sharing an environment.
- Revoke unused service API keys.
- Use unique API keys per service.
- Store secrets in a secret manager.
- Do not log plaintext API keys.
- Treat `memory://local` blockchain and `memory://zkp` as development adapters.

Before production use, UDDI still needs a full security audit, formal threat model, real ZKP prover/verifier integration, and real decentralized registry integration.

## Troubleshooting

### `lookup postgres ... network is unreachable`

This is usually Docker DNS/network timing during startup. The API retries database connection automatically. If it keeps failing:

```bash
docker compose -f infra/docker/docker-compose.dev.yml down -v --remove-orphans
docker compose -f infra/docker/docker-compose.dev.yml up -d --build
```

### `role "uddi" does not exist`

You are probably reusing an old Postgres volume created with different credentials. Recreate the dev volume:

```bash
docker compose -f infra/docker/docker-compose.dev.yml down -v --remove-orphans
docker compose -f infra/docker/docker-compose.dev.yml up -d --build
```

### `/docs` returns 404

Rebuild and recreate the API container:

```bash
docker compose -f infra/docker/docker-compose.dev.yml up -d --build
```

Then verify from inside the container:

```bash
docker exec uddi-framework-api-1 wget -qO- http://127.0.0.1:8080/docs
```
