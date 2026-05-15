# UDDI E2E Smoke

End-to-end smoke test for the local UDDI Docker API and workspace SDK packages.

It verifies:

- API readiness
- DID registration and resolution
- DID key rotation
- Auth challenge and verification
- Credential issue, list, verify, and revoke
- Health, metrics, OpenAPI, and Swagger UI endpoints

## Run

Start the local stack first:

```bash
pnpm docker:dev
```

Then run:

```bash
pnpm e2e:smoke
```

Optional environment:

```bash
UDDI_API_URL=http://localhost:8080 \
UDDI_ADMIN_TOKEN=dev-admin-token-change-in-production \
pnpm e2e:smoke
```
