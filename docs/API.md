# UDDI API Contract

This document describes the current alpha REST API exposed by `packages/api`.
The local examples assume the API is running on `http://localhost:8080`.

For tooling and generated clients, see the OpenAPI 3.1 contract in [`openapi.yaml`](./openapi.yaml). When the API is running, the same contract is available at `GET /openapi.yaml`, and Swagger UI is available at `GET /docs`.

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

If the per-IP rate limit is exceeded, the API responds with:

```json
{
  "error": "rate limit exceeded"
}
```

Admin endpoints require:

```http
X-Admin-Token: dev-admin-token-change-in-production
Content-Type: application/json
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
  "version": "0.1.1"
}
```

### `GET /ready`

```bash
curl http://localhost:8080/ready
```

Response:

```json
{
  "status": "ready",
  "version": "0.1.1"
}
```

### `GET /metrics`

Returns lightweight JSON runtime metrics.

```bash
curl http://localhost:8080/metrics
```

Response:

```json
{
  "startedAt": "2026-05-14T04:00:00Z",
  "uptimeSeconds": 60,
  "requestsTotal": 42,
  "responsesTotal": 41,
  "errorsTotal": 0,
  "inFlightRequests": 1,
  "latencyTotalMs": 120,
  "latencyAverageMs": 2.92,
  "metricsContentType": "application/json"
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

Rotates the DID public key and optionally updates the DID document context. The request must be signed by the DID's current public key over:

```text
update:{did}:{newPublicKeyBase64}:{timestamp}
```

Request:

```bash
curl -X PUT 'http://localhost:8080/v1/did/did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123/update' \
  -H 'Content-Type: application/json' \
  -d '{
    "did": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
    "publicKeyBase64": "BASE64_NEW_ED25519_PUBLIC_KEY",
    "context": [
      "https://www.w3.org/ns/did/v1",
      "https://uddi.network/v1"
    ],
    "signatureBase64": "BASE64_SIGNATURE_FROM_CURRENT_KEY",
    "timestamp": "2026-05-14T04:20:00Z"
  }'
```

Response:

```json
{
  "did": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
  "txHash": "0x...",
  "status": "UPDATED",
  "updatedAt": "2026-05-14T04:20:00Z"
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

These endpoints are protected by API key middleware. The API stores already-issued credentials as registry records. Issuer-side signing is still performed outside the API, for example by `@uddi/core`; the API verifies the Ed25519 credential proof before storing and during status verification.

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
  "credentials": [
    {
      "id": "urn:uddi:vc:example",
      "issuer": "did:uddi:zIssuerIdentifierWithAtLeastFortyCharacters123",
      "subject": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
      "types": ["VerifiableCredential", "AgeCredential"],
      "credential": {
        "@context": ["https://www.w3.org/2018/credentials/v1"],
        "id": "urn:uddi:vc:example",
        "type": ["VerifiableCredential", "AgeCredential"],
        "issuer": "did:uddi:zIssuerIdentifierWithAtLeastFortyCharacters123",
        "issuanceDate": "2026-05-14T04:00:00Z",
        "credentialSubject": {
          "id": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
          "birthYear": 2000
        },
        "proof": {
          "type": "Ed25519Signature2020",
          "verificationMethod": "did:uddi:zIssuerIdentifierWithAtLeastFortyCharacters123#keys-1",
          "proofPurpose": "assertionMethod",
          "proofValue": "BASE64_SIGNATURE"
        }
      },
      "issuanceDate": "2026-05-14T04:00:00Z",
      "createdAt": "2026-05-14T04:00:01Z"
    }
  ]
}
```

### `POST /v1/credentials/issue`

Stores a signed Verifiable Credential in the registry. The credential issuer DID and subject DID must already exist in the DID registry. The API verifies `proof.proofValue` against the issuer DID public key over the canonical credential payload without the `proof` field.

`issuanceDate` must be RFC3339. When `expirationDate` is present, it must also be RFC3339 and later than `issuanceDate`.

Request:

```bash
curl -X POST http://localhost:8080/v1/credentials/issue \
  -H 'Content-Type: application/json' \
  -H 'X-Service-ID: dev-service' \
  -H 'X-API-Key: dev-api-key' \
  -d '{
    "credential": {
      "@context": ["https://www.w3.org/2018/credentials/v1"],
      "id": "urn:uddi:vc:example",
      "type": ["VerifiableCredential", "AgeCredential"],
      "issuer": "did:uddi:zIssuerIdentifierWithAtLeastFortyCharacters123",
      "issuanceDate": "2026-05-14T04:00:00Z",
      "expirationDate": "2027-05-14T04:00:00Z",
      "credentialSubject": {
        "id": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
        "birthYear": 2000
      },
      "proof": {
        "type": "Ed25519Signature2020",
        "verificationMethod": "did:uddi:zIssuerIdentifierWithAtLeastFortyCharacters123#keys-1",
        "proofPurpose": "assertionMethod",
        "proofValue": "BASE64_SIGNATURE"
      }
    }
  }'
```

Response `201`:

```json
{
  "status": "ISSUED",
  "credential": {
    "id": "urn:uddi:vc:example",
    "issuer": "did:uddi:zIssuerIdentifierWithAtLeastFortyCharacters123",
    "subject": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
    "types": ["VerifiableCredential", "AgeCredential"],
    "issuanceDate": "2026-05-14T04:00:00Z",
    "createdAt": "2026-05-14T04:00:01Z"
  }
}
```

### `POST /v1/credentials/revoke`

Request:

```bash
curl -X POST http://localhost:8080/v1/credentials/revoke \
  -H 'Content-Type: application/json' \
  -H 'X-Service-ID: dev-service' \
  -H 'X-API-Key: dev-api-key' \
  -d '{
    "id": "urn:uddi:vc:example",
    "reason": "issuer requested revocation"
  }'
```

Response:

```json
{
  "status": "REVOKED",
  "id": "urn:uddi:vc:example"
}
```

### `GET /v1/credentials/{id}/verify`

Checks registry status, expiration, and credential proof. Revoked credentials return `valid: false` with `reason: "credential revoked"`. Expired credentials return `valid: false` with `reason: "credential expired"`.

Response:

```json
{
  "id": "urn:uddi:vc:example",
  "valid": true,
  "reason": "",
  "issuer": "did:uddi:zIssuerIdentifierWithAtLeastFortyCharacters123",
  "subject": "did:uddi:zExampleIdentifierWithAtLeastFortyCharacters123",
  "types": ["VerifiableCredential", "AgeCredential"],
  "verifiedAt": "2026-05-14T04:00:02Z"
}
```

## API Key Management

These endpoints manage service API keys. They require `X-Admin-Token` and never return stored API key hashes. A plaintext API key is returned only once when created.

### `POST /v1/admin/api-keys/`

Request:

```bash
curl -X POST http://localhost:8080/v1/admin/api-keys/ \
  -H 'Content-Type: application/json' \
  -H 'X-Admin-Token: dev-admin-token-change-in-production' \
  -d '{
    "serviceId": "merchant-service",
    "serviceName": "Merchant Service"
  }'
```

Response `201`:

```json
{
  "apiKey": "uddi_GENERATED_SECRET",
  "record": {
    "serviceId": "merchant-service",
    "serviceName": "Merchant Service",
    "createdAt": "2026-05-14T04:03:00Z"
  }
}
```

### `GET /v1/admin/api-keys/`

```bash
curl http://localhost:8080/v1/admin/api-keys/ \
  -H 'X-Admin-Token: dev-admin-token-change-in-production'
```

Response:

```json
{
  "apiKeys": [
    {
      "serviceId": "merchant-service",
      "serviceName": "Merchant Service",
      "createdAt": "2026-05-14T04:03:00Z"
    }
  ]
}
```

### `POST /v1/admin/api-keys/revoke`

```bash
curl -X POST http://localhost:8080/v1/admin/api-keys/revoke \
  -H 'Content-Type: application/json' \
  -H 'X-Admin-Token: dev-admin-token-change-in-production' \
  -d '{
    "serviceId": "merchant-service"
  }'
```

Response:

```json
{
  "status": "REVOKED",
  "serviceId": "merchant-service"
}
```

## Proof And Claim Verification

### `POST /v1/proof/generate`

Generates a ZKP-shaped response through the configured ZKP adapter. `memory://zkp` uses a local development proof shape for contract testing. `http://` or `https://` URLs delegate to an external prover at `/generate`.

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
    "circuit": "age_verification",
    "mode": "development",
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

Verifies a ZKP-shaped proof through the configured ZKP adapter. `memory://zkp` validates the expected Groth16 proof envelope and claim type; `http://` or `https://` URLs delegate to an external verifier at `/verify`.

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
      "curve": "bn128",
      "type": "age_verification",
      "proof": {
        "pi_a": [],
        "pi_b": [],
        "pi_c": []
      },
      "publicSignals": []
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
  "activeDIDs": 0,
  "deactivatedDIDs": 0,
  "backend": "memory"
}
```
