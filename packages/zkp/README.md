# UDDI ZKP Circuits

Early Circom circuit drafts for privacy-preserving claim verification.

## Circuits

- `circuits/age_verification.circom`
  Proves a holder is at least a requested age without revealing their birth date.

- `circuits/citizenship_verification.circom`
  Proves a holder has citizenship for a requested country code without revealing their national ID.

## Current Status

These circuits are drafts. The repository does not yet include a prover/verifier service, trusted setup artifacts, generated proving keys, or automated circuit tests.

## Roadmap

- Add circuit compile/test scripts.
- Add fixture inputs.
- Add proving and verifying key generation.
- Add Go or Node service wrapper.
- Connect `packages/api` proof endpoints to the real verifier.
