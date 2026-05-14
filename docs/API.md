# UDDI API Contract

This document describes the current alpha REST API exposed by `packages/api`.
The local examples assume the API is running on `http://localhost:8080`.

Protected endpoints require these headers:

```http
X-Service-ID: dev-service
X-API-Key: dev-api-key
Content-Type: application/json
```

The development API key store seeds two credentials in memory and Postgres:

| Service ID | API Key |
| --- | --- |
| `dev-service` | `dev-api-key` |
| `test-service` | `test-key` |

Error responses use this shape:

```json
{
  "error": "message"
}
```

## Health

### `GET /health`

```bash
curl http://localhost:8080/health
```

Response:

```json
{
  "status": "ok",
  "version": "0.1.0"
}
```

## DID Registry

### `POST /v1/did/register`

Registers a `did:uddi:z...` identifier. The signature must be Ed25519 over:

```text
register:{did}:{timestamp}
```

Request:

```bash
curl -X POST http://localhost:8080/v1/did/register \
  -H 'Content-Type: application/json' \
  -d '{
    "did": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
    "publicKeyBase64": "BASE64_ED25519_PUBLIC_KEY",
    "signatureBase64": "BASE64_SIGNATURE",
    "timestamp": "2026-05-14T04:00:00Z"
  }'
```

Response `201`:

```json
{
  "did": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
  "txHash": "0x...",
  "status": "REGISTERED",
  "createdAt": "2026-05-14T04:00:00Z"
}
```

### `GET /v1/did/{did}`

Resolves a registered DID document.

```bash
curl 'http://localhost:8080/v1/did/did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123'
```

Response:

```json
{
  "didDocument": {
    "@context": [
      "https://www.w3.org/ns/did/v1"
    ],
    "id": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
    "publicKeyBase64": "BASE64_ED25519_PUBLIC_KEY",
    "created": "2026-05-14T04:00:00Z",
    "updated": "2026-05-14T04:00:00Z",
    "deactivated": false
  },
  "didDocumentMetadata": {
    "created": "2026-05-14T04:00:00Z",
    "updated": "2026-05-14T04:00:00Z",
    "deactivated": false
  },
  "didResolutionMetadata": {
    "contentType": "application/did+ld+json"
  }
}
```

### `POST /v1/did/revoke`

Revokes a registered DID. The signature must be Ed25519 over:

```text
revoke:{did}:{timestamp}
```

Request:

```bash
curl -X POST http://localhost:8080/v1/did/revoke \
  -H 'Content-Type: application/json' \
  -d '{
    "did": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
    "signatureBase64": "BASE64_SIGNATURE",
    "timestamp": "2026-05-14T04:10:00Z"
  }'
```

Response:

```json
{
  "status": "REVOKED",
  "did": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123"
}
```

### `PUT /v1/did/{did}/update`

This endpoint exists but is not implemented yet.

Response `501`:

```json
{
  "error": "DID update not yet implemented"
}
```

## Authentication Verification

### `POST /v1/verify/challenge`

Creates a short-lived, single-use authentication challenge for a verifier service.

Request:

```bash
curl -X POST http://localhost:8080/v1/verify/challenge \
  -H 'Content-Type: application/json' \
  -H 'X-Service-ID: dev-service' \
  -H 'X-API-Key: dev-api-key' \
  -d '{
    "serviceId": "dev-service",
    "serviceName": "UDDI Dev Service"
  }'
```

Response `201`:

```json
{
  "challengeId": "RANDOM_CHALLENGE_ID",
  "nonce": "RANDOM_NONCE",
  "serviceId": "dev-service",
  "serviceName": "UDDI Dev Service",
  "issuedAt": "2026-05-14T04:00:00Z",
  "expiresAt": "2026-05-14T04:05:00Z",
  "qrCode": "uddi://auth?challengeId=RANDOM_CHALLENGE_ID"
}
```

### `POST /v1/verify/auth`

Verifies a holder authentication presentation. The presentation is a base64-encoded JSON payload:

```json
{
  "did": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
  "challengeId": "RANDOM_CHALLENGE_ID",
  "signature": "BASE64_SIGNATURE",
  "timestamp": 1778731200000
}
```

The signature must be Ed25519 over:

```text
{challengeId}:{nonce}:{did}:{timestamp}
```

The timestamp is Unix milliseconds. The API rejects presentations older than five minutes or more than thirty seconds in the future.

Request:

```bash
curl -X POST http://localhost:8080/v1/verify/auth \
  -H 'Content-Type: application/json' \
  -H 'X-Service-ID: dev-service' \
  -H 'X-API-Key: dev-api-key' \
  -d '{
    "challengeId": "RANDOM_CHALLENGE_ID",
    "serviceId": "dev-service",
    "presentation": "BASE64_JSON_PRESENTATION"
  }'
```

Response:

```json
{
  "valid": true,
  "did": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
  "verifiedAt": "2026-05-14T04:01:00Z",
  "verifiedClaims": []
}
```

Invalid presentations still return `200` with `valid: false`:

```json
{
  "valid": false,
  "did": "",
  "verifiedAt": "2026-05-14T04:01:00Z",
  "verifiedClaims": [],
  "reason": "challenge not found or presentation missing"
}
```

## Credential Endpoints

These endpoints are protected by API key middleware. The registry behavior is still placeholder-level in the alpha API.

### `GET /v1/credentials/{did}`

```bash
curl 'http://localhost:8080/v1/credentials/did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123' \
  -H 'X-Service-ID: dev-service' \
  -H 'X-API-Key: dev-api-key'
```

Response:

```json
{
  "did": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
  "credentials": []
}
```

### `POST /v1/credentials/issue`

Response `202`:

```json
{
  "status": "PENDING_IMPLEMENTATION"
}
```

### `POST /v1/credentials/revoke`

Response `202`:

```json
{
  "status": "PENDING_IMPLEMENTATION"
}
```

### `GET /v1/credentials/{id}/verify`

Response:

```json
{
  "id": "credential-id",
  "valid": false,
  "reason": "credential registry not implemented yet"
}
```

## Proof And Claim Verification

### `POST /v1/proof/generate`

Generates a ZKP-shaped stub response today. Runtime prover integration is still roadmap.

Request:

```bash
curl -X POST http://localhost:8080/v1/proof/generate \
  -H 'Content-Type: application/json' \
  -d '{
    "did": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
    "type": "age_verification",
    "params": {
      "minimumAge": 18
    }
  }'
```

Response:

```json
{
  "proof": {
    "protocol": "groth16",
    "curve": "bn128",
    "type": "age_verification",
    "params": {
      "minimumAge": 18
    },
    "serviceUrl": "memory://zkp",
    "proof": {
      "pi_a": [],
      "pi_b": [],
      "pi_c": []
    },
    "publicSignals": []
  }
}
```

### `POST /v1/verify/claim`

Verifies a ZKP-shaped proof through the current stub service. It returns `valid: true` when `claimType` is non-empty and `proof` is present.

Request:

```bash
curl -X POST http://localhost:8080/v1/verify/claim \
  -H 'Content-Type: application/json' \
  -H 'X-Service-ID: dev-service' \
  -H 'X-API-Key: dev-api-key' \
  -d '{
    "did": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
    "claimType": "age_verification",
    "proof": {
      "protocol": "groth16",
      "proof": {}
    }
  }'
```

Response:

```json
{
  "valid": true,
  "claimType": "age_verification",
  "verifiedAt": "2026-05-14T04:02:00Z",
  "publicClaims": {}
}
```

## Registry Stats

### `GET /v1/registry/stats`

```bash
curl http://localhost:8080/v1/registry/stats
```

Response:

```json
{
  "totalDIDs": 0,
  "activeCredentials": 0,
  "revokedCredentials": 0
}
```
