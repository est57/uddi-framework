import { afterEach, describe, expect, it, vi } from 'vitest';
import { generateIdentity } from '@uddi/core/identity';
import type { UddiIdentity, VerifiableCredential } from '@uddi/core/types';
import { UddiClient, UddiVerifier, type IdentityStorage } from './index';

function mockFetch(payload: unknown = {}) {
  const fetchMock = vi.fn(async () => new Response(JSON.stringify(payload), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  }));
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

function createMemoryStorage(identity?: UddiIdentity): IdentityStorage {
  let stored = identity ?? null;
  return {
    async save(nextIdentity) {
      stored = nextIdentity;
    },
    async load() {
      return stored;
    },
    async delete() {
      stored = null;
    },
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('UddiClient', () => {
  it('creates an identity and registers it with the API', async () => {
    const fetchMock = mockFetch({ txHash: '0xabc' });
    const storage = createMemoryStorage();
    const client = new UddiClient({ network: 'local', storage });

    const identity = await client.createIdentity();

    expect(identity.did).toMatch(/^did:uddi:z/);
    expect(client.getDid()).toBe(identity.did);
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock.mock.calls[0]?.[0]).toBe('http://localhost:8080/v1/did/register');

    const request = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
    expect(request.did).toBe(identity.did);
    expect(request.publicKeyBase64).toEqual(expect.any(String));
    expect(request.signatureBase64).toEqual(expect.any(String));
  });

  it('loads an identity from configured storage', async () => {
    const storedIdentity = await generateIdentity();
    const client = new UddiClient({
      network: 'local',
      storage: createMemoryStorage(storedIdentity),
    });

    await expect(client.loadIdentity()).resolves.toEqual({ did: storedIdentity.did });
    expect(client.getDid()).toBe(storedIdentity.did);
  });

  it('generates proofs for the loaded identity', async () => {
    const storedIdentity = await generateIdentity();
    const fetchMock = mockFetch({
      proof: {
        protocol: 'groth16',
        curve: 'bn128',
        proof: { pi_a: [], pi_b: [], pi_c: [] },
        publicSignals: [],
      },
    });
    const client = new UddiClient({
      network: 'local',
      storage: createMemoryStorage(storedIdentity),
    });

    await client.loadIdentity();
    await client.generateProof({ type: 'age', params: { minimumAge: 18 } });

    expect(fetchMock.mock.calls[0]?.[0]).toBe('http://localhost:8080/v1/proof/generate');
    const request = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
    expect(request).toMatchObject({
      did: storedIdentity.did,
      type: 'age',
      params: { minimumAge: 18 },
    });
  });

  it('revokes the loaded identity using the API contract expected by Go', async () => {
    const storedIdentity = await generateIdentity();
    const fetchMock = mockFetch({ status: 'REVOKED', did: storedIdentity.did });
    const storage = createMemoryStorage(storedIdentity);
    const client = new UddiClient({
      network: 'local',
      storage,
    });

    await client.loadIdentity();
    await client.revokeIdentity();

    expect(fetchMock.mock.calls[0]?.[0]).toBe('http://localhost:8080/v1/did/revoke');
    const request = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
    expect(request).toMatchObject({
      did: storedIdentity.did,
      timestamp: expect.any(String),
      signatureBase64: expect.any(String),
    });
    expect(request).not.toHaveProperty('signature');
    await expect(storage.load()).resolves.toBeNull();
    expect(client.getDid()).toBeNull();
  });

  it('rotates the loaded identity key and keeps the DID stable', async () => {
    const storedIdentity = await generateIdentity();
    const fetchMock = mockFetch({
      did: storedIdentity.did,
      txHash: '0xupdate',
      status: 'UPDATED',
      updatedAt: new Date().toISOString(),
    });
    const storage = createMemoryStorage(storedIdentity);
    const client = new UddiClient({
      network: 'local',
      storage,
    });

    await client.loadIdentity();
    const result = await client.rotateIdentityKey(['https://example.com/context']);

    expect(result).toMatchObject({
      did: storedIdentity.did,
      txHash: '0xupdate',
      status: 'UPDATED',
    });
    expect(client.getDid()).toBe(storedIdentity.did);
    expect(fetchMock.mock.calls[0]?.[0]).toBe(`http://localhost:8080/v1/did/${storedIdentity.did}/update`);
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe('PUT');

    const request = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
    expect(request).toMatchObject({
      did: storedIdentity.did,
      context: ['https://example.com/context'],
      publicKeyBase64: expect.any(String),
      timestamp: expect.any(String),
      signatureBase64: expect.any(String),
    });

    const rotatedIdentity = await storage.load();
    expect(rotatedIdentity?.did).toBe(storedIdentity.did);
    expect(rotatedIdentity?.publicKey).not.toEqual(storedIdentity.publicKey);
    expect(rotatedIdentity?.privateKey).not.toEqual(storedIdentity.privateKey);
  });

  it('keeps the previous key if identity key rotation is rejected', async () => {
    const storedIdentity = await generateIdentity();
    const fetchMock = vi.fn(async () => jsonResponse({ error: 'invalid signature' }, 401));
    vi.stubGlobal('fetch', fetchMock);
    const storage = createMemoryStorage(storedIdentity);
    const client = new UddiClient({
      network: 'local',
      storage,
    });

    await client.loadIdentity();

    await expect(client.rotateIdentityKey()).rejects.toThrow('UDDI API error: invalid signature');
    const identityAfterFailure = await storage.load();
    expect(identityAfterFailure).toEqual(storedIdentity);
    expect(client.getDid()).toBe(storedIdentity.did);
  });
});

describe('UddiVerifier', () => {
  it('sends API key headers when creating challenges', async () => {
    const fetchMock = mockFetch({
      challengeId: 'challenge-1',
      nonce: 'nonce-1',
      serviceId: 'service-1',
      serviceName: 'Service One',
      issuedAt: new Date().toISOString(),
      expiresAt: new Date().toISOString(),
    });
    const verifier = new UddiVerifier({
      network: 'local',
      serviceId: 'service-1',
      serviceName: 'Service One',
      apiKey: 'secret',
    });

    await verifier.createAuthChallenge();

    expect(fetchMock.mock.calls[0]?.[0]).toBe('http://localhost:8080/v1/verify/challenge');
    expect(fetchMock.mock.calls[0]?.[1]?.headers).toMatchObject({
      'Content-Type': 'application/json',
      'X-API-Key': 'secret',
      'X-Service-ID': 'service-1',
    });
  });

  it('rejects invalid DID claim verification before calling the API', async () => {
    const fetchMock = mockFetch();
    const verifier = new UddiVerifier({
      network: 'local',
      serviceId: 'service-1',
      apiKey: 'secret',
    });

    await expect(verifier.verifyClaim('not-a-did' as never, {
      type: 'age',
      proof: {
        protocol: 'groth16',
        curve: 'bn128',
        proof: { pi_a: [], pi_b: [], pi_c: [] },
        publicSignals: [],
      },
    })).rejects.toThrow('Invalid DID format');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('submits signed credentials with API key headers', async () => {
    const credential = sampleCredential();
    const fetchMock = mockFetch({
      status: 'ISSUED',
      credential: {
        id: credential.id,
        issuer: credential.issuer,
        subject: credential.credentialSubject.id,
        types: credential.type,
        credential,
        issuanceDate: credential.issuanceDate,
        createdAt: new Date().toISOString(),
      },
    });
    const verifier = new UddiVerifier({
      network: 'local',
      serviceId: 'service-1',
      apiKey: 'secret',
    });

    const record = await verifier.submitCredential(credential);

    expect(record.id).toBe(credential.id);
    expect(fetchMock.mock.calls[0]?.[0]).toBe('http://localhost:8080/v1/credentials/issue');
    expect(fetchMock.mock.calls[0]?.[1]?.headers).toMatchObject({
      'X-API-Key': 'secret',
      'X-Service-ID': 'service-1',
    });
    const request = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
    expect(request.credential).toEqual(credential);
  });

  it('lists, verifies, and revokes credential registry records', async () => {
    const credential = sampleCredential();
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith(`/v1/credentials/${credential.credentialSubject.id}`)) {
        return jsonResponse({ did: credential.credentialSubject.id, credentials: [{ id: credential.id }] });
      }
      if (url.endsWith(`/v1/credentials/${encodeURIComponent(credential.id)}/verify`)) {
        return jsonResponse({
          id: credential.id,
          valid: true,
          reason: '',
          verifiedAt: new Date().toISOString(),
        });
      }
      if (url.endsWith('/v1/credentials/revoke')) {
        return jsonResponse({ status: 'REVOKED', id: credential.id });
      }
      return jsonResponse({ error: 'not found' }, 404);
    });
    vi.stubGlobal('fetch', fetchMock);
    const verifier = new UddiVerifier({
      network: 'local',
      serviceId: 'service-1',
      apiKey: 'secret',
    });

    await expect(verifier.listCredentials(credential.credentialSubject.id)).resolves.toHaveLength(1);
    await expect(verifier.verifyCredentialStatus(credential.id)).resolves.toMatchObject({
      id: credential.id,
      valid: true,
    });
    await expect(verifier.revokeCredential(credential.id, 'test')).resolves.toEqual({
      status: 'REVOKED',
      id: credential.id,
    });

    expect(fetchMock.mock.calls[2]?.[0]).toBe('http://localhost:8080/v1/credentials/revoke');
    const revokeBody = JSON.parse(String(fetchMock.mock.calls[2]?.[1]?.body));
    expect(revokeBody).toEqual({ id: credential.id, reason: 'test' });
  });
});

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function sampleCredential(): VerifiableCredential<{ birthYear: number }> {
  return {
    '@context': ['https://www.w3.org/2018/credentials/v1'],
    id: 'urn:uddi:vc:sample',
    type: ['VerifiableCredential', 'AgeCredential'],
    issuer: 'did:uddi:zissuer123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij',
    issuanceDate: '2026-05-14T00:00:00Z',
    credentialSubject: {
      id: 'did:uddi:zsubject12345678ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij',
      birthYear: 2000,
    },
    proof: {
      type: 'Ed25519Signature2020',
      created: '2026-05-14T00:00:00Z',
      verificationMethod: 'did:uddi:zissuer123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij#keys-1',
      proofPurpose: 'assertionMethod',
      proofValue: 'signature',
    },
  };
}
