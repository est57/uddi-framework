# Changelog

All notable changes to UDDI Framework are documented in this file.

UDDI is still in alpha. Minor versions can include API and SDK contract changes while the production surface is being stabilized.

## 0.1.1 - 2026-05-15

### Added

- Added `UddiClient.rotateIdentityKey()` to rotate a DID signing key while keeping the DID stable.
- Added Docker-backed E2E smoke testing in `examples/e2e-smoke`.
- Added manual `E2E Smoke` GitHub Actions workflow, also triggered for release tags.
- Added deployment guidance in `docs/DEPLOYMENT.md`.
- Added OpenAPI serving at `GET /openapi.yaml`.
- Added Swagger UI at `GET /docs`.

### Fixed

- Fixed SDK credential status lookup for credential IDs such as `urn:uddi:vc:...`.
- Improved Docker startup reliability by retrying initial DID database connection.

### Verified

- Published `@uddi/core@0.1.1`.
- Published `@uddi/sdk@0.1.1`.
- Verified the published npm quickstart against `@uddi/core@0.1.1` and `@uddi/sdk@0.1.1`.
- Verified local Docker API + SDK flow with `pnpm e2e:smoke`.

## 0.1.0 - 2026-05-14

### Added

- Initial alpha release of the UDDI decentralized identity framework.
- TypeScript core identity, DID, signing, and Verifiable Credential primitives.
- TypeScript SDK for holder and verifier flows.
- Go API gateway with DID registry, auth verification, credential registry, admin API key management, and observability endpoints.
- Postgres-backed DID, API key, and credential stores.
- Redis-backed auth challenge store.
- Versioned Postgres migrations.
- Docker development stack.
- GitHub CI and release validation workflows.
