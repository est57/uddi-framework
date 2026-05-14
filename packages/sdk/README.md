# @uddi/sdk

Developer-facing TypeScript SDK for integrating UDDI holder and verifier flows.

## Main Classes

- `UddiClient`: holder/user-side identity operations.
- `UddiVerifier`: application/verifier-side auth and claim verification calls.

## Current Capabilities

- Create and load identities.
- Register identities through the API.
- Sign auth challenges.
- Request proof generation through the API.
- Resolve DIDs.
- Create auth challenges.
- Verify auth presentations.
- Verify ZK claim payloads through the API.

## Build And Test

From the repository root:

```bash
pnpm --filter @uddi/sdk build
pnpm --filter @uddi/sdk test
pnpm --filter @uddi/sdk lint
```

## Example

```typescript
import { UddiClient, UddiVerifier } from '@uddi/sdk';

const client = new UddiClient({ network: 'local' });
const { did } = await client.createIdentity();

const verifier = new UddiVerifier({
  network: 'local',
  serviceId: 'example-app',
  apiKey: 'dev-key',
});

const challenge = await verifier.createAuthChallenge();
const presentation = await client.authenticate(challenge);
const result = await verifier.verifyAuth(challenge.challengeId, presentation);

console.log(did, result.valid);
```

## Notes

`UddiClient` accepts a custom `IdentityStorage` implementation. Production apps should use secure storage appropriate to the platform.
