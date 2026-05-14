# UDDI npm Quickstart

This example uses the published npm packages:

- `@uddi/core`
- `@uddi/sdk`

It does not depend on workspace package links, so it behaves like a fresh external consumer project.

## Run

From the repository root:

```bash
pnpm install
pnpm --filter @uddi/example-npm-quickstart start
```

Expected output includes:

```text
DID: did:uddi:z...
Signature valid: true
SDK exports: function function
```
