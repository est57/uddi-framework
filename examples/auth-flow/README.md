# UDDI Auth Flow Example

This example runs the current end-to-end authentication flow against a local UDDI API:

1. Create a holder identity.
2. Register the DID through the API.
3. Create a verifier challenge.
4. Sign the challenge from the holder side.
5. Verify the authentication presentation.
6. Generate and verify a ZKP-shaped claim stub.

## Run

Start the API stack from the repository root:

```bash
docker compose -f infra/docker/docker-compose.dev.yml up --build
```

Build the SDK packages:

```bash
pnpm build
```

Run the example:

```bash
pnpm example:auth-flow
```

You can override the default local settings:

```bash
UDDI_API_URL=http://localhost:8080 \
UDDI_SERVICE_ID=dev-service \
UDDI_API_KEY=dev-api-key \
pnpm example:auth-flow
```

## Expected Output

The final authentication and claim verification results should both print `valid: true`.
