import { describe, expect, it } from 'vitest';
import {
  createAuthPresentation,
  createDidDocument,
  didEquals,
  didToPublicKeyHash,
  exportPublicIdentity,
  generateIdentity,
  isValidDid,
  publicKeyToDid,
  signMessage,
  verifySignature,
} from './index';

describe('identity', () => {
  it('generates a valid UDDI identity', async () => {
    const identity = await generateIdentity();

    expect(identity.did).toMatch(/^did:uddi:z/);
    expect(isValidDid(identity.did)).toBe(true);
    expect(identity.privateKey).toHaveLength(32);
    expect(identity.publicKey).toHaveLength(32);
    expect(publicKeyToDid(identity.publicKey)).toBe(identity.did);
    expect(didToPublicKeyHash(identity.did)).toHaveLength(32);
  });

  it('signs and verifies messages', async () => {
    const identity = await generateIdentity();
    const signature = await signMessage('hello uddi', identity.privateKey);

    expect(await verifySignature('hello uddi', signature, identity.publicKey)).toBe(true);
    expect(await verifySignature('tampered', signature, identity.publicKey)).toBe(false);
  });

  it('exports public identity and creates DID document', async () => {
    const identity = await generateIdentity();
    const publicIdentity = exportPublicIdentity(identity);
    const didDocument = createDidDocument(publicIdentity);

    expect(publicIdentity).not.toHaveProperty('privateKey');
    expect(didDocument.id).toBe(identity.did);
    expect(didDocument.authentication).toContain(`${identity.did}#keys-1`);
    expect(didDocument.verificationMethod[0]?.controller).toBe(identity.did);
  });

  it('creates a signed auth presentation payload', async () => {
    const identity = await generateIdentity();
    const presentationBase64 = await createAuthPresentation('challenge-1', 'nonce-1', identity);
    const presentation = JSON.parse(Buffer.from(presentationBase64, 'base64').toString('utf8'));

    expect(presentation.did).toBe(identity.did);
    expect(presentation.challengeId).toBe('challenge-1');
    expect(typeof presentation.signature).toBe('string');
    expect(typeof presentation.timestamp).toBe('number');
  });

  it('compares DIDs case-sensitively', async () => {
    const identity = await generateIdentity();

    expect(didEquals(identity.did, identity.did)).toBe(true);
    expect(didEquals(identity.did, `${identity.did}x` as typeof identity.did)).toBe(false);
  });
});
