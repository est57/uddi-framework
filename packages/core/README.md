# @uddi/core

Core TypeScript primitives for the UDDI framework.

## Install

```bash
pnpm add @uddi/core
```

## What It Provides

- Ed25519 identity generation.
- DID derivation from public keys.
- DID document creation.
- Message signing and signature verification.
- Auth presentation creation.
- Verifiable Credential issuance, verification, hashing, and presentation creation.
- DID/VC structural compliance baseline tests.
- Shared protocol types.

## Build And Test

From the repository root:

```bash
pnpm --filter @uddi/core build
pnpm --filter @uddi/core test
pnpm --filter @uddi/core lint
```

## Example

```typescript
import {
  generateIdentity,
  signMessage,
  verifySignature,
} from '@uddi/core';

const identity = await generateIdentity();
const signature = await signMessage('hello', identity.privateKey);
const valid = await verifySignature('hello', signature, identity.publicKey);

console.log(identity.did, valid);
```

## Notes

The private key is represented as raw bytes and must be stored by the caller using platform-appropriate secure storage. This package does not provide secure key storage.
