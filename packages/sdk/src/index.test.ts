import { afterEach, describe, expect, it, vi } from 'vitest';
import { generateIdentity } from '@uddi/core/identity';
import type { UddiIdentity } from '@uddi/core/types';
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
});
