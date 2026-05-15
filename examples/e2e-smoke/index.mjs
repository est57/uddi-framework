import { issueCredential } from '@uddi/core/vc';
import { UddiClient, UddiVerifier } from '@uddi/sdk';

const apiUrl = process.env.UDDI_API_URL ?? 'http://localhost:8080';
const adminToken = process.env.UDDI_ADMIN_TOKEN ?? 'dev-admin-token-change-in-production';

const storage = createMemoryStorage();

await waitForReady();

const apiKey = await createSmokeAPIKey();
const verifier = new UddiVerifier({
  apiUrl,
  network: 'local',
  serviceId: apiKey.serviceId,
  serviceName: apiKey.serviceName,
  apiKey: apiKey.apiKey,
});

const client = new UddiClient({
  apiUrl,
  network: 'local',
  storage,
});

const { did } = await client.createIdentity();
const firstDocument = await resolveDID(did);

const update = await client.rotateIdentityKey(['https://www.w3.org/ns/did/v1', 'https://uddi.network/v1']);
assert(update.status === 'UPDATED', 'DID update should return UPDATED');

const rotatedDocument = await resolveDID(did);
assert(rotatedDocument.didDocument.id === did, 'Resolved DID document should match the holder DID');
assert(
  rotatedDocument.didDocument.publicKeyBase64 !== firstDocument.didDocument.publicKeyBase64,
  'DID public key should change after rotation',
);

const challenge = await verifier.createAuthChallenge();
const presentation = await client.authenticate(challenge);
const auth = await verifier.verifyAuth(challenge.challengeId, presentation);
assert(auth.valid === true, 'Auth verification should be valid after key rotation');
assert(auth.did === did, 'Auth verification should return the holder DID');

const holderIdentity = await storage.load();
assert(holderIdentity, 'Holder identity should be stored');

const credential = await issueCredential(
  holderIdentity,
  did,
  'KycCredential',
  {
    level: 'basic',
    country: 'ID',
  },
  1,
);

const record = await verifier.submitCredential(credential);
assert(record.id === credential.id, 'Submitted credential id should match');

const credentials = await verifier.listCredentials(did);
assert(credentials.some(item => item.id === credential.id), 'Credential registry should list submitted credential');

const credentialStatus = await verifier.verifyCredentialStatus(credential.id);
assert(
  credentialStatus.valid === true,
  `Credential status should be valid before revocation: ${formatPayload(credentialStatus)}`,
);

const revoked = await verifier.revokeCredential(credential.id, 'e2e smoke cleanup');
assert(revoked.status === 'REVOKED', 'Credential revocation should return REVOKED');

const revokedStatus = await verifier.verifyCredentialStatus(credential.id);
assert(
  revokedStatus.valid === false,
  `Credential status should be invalid after revocation: ${formatPayload(revokedStatus)}`,
);

await expectOK('/health');
await expectOK('/ready');
await expectOK('/metrics');
await expectOK('/openapi.yaml');
await expectOK('/docs');

console.log('UDDI E2E smoke passed');
console.log(`DID: ${did}`);
console.log(`Credential: ${credential.id}`);

async function createSmokeAPIKey() {
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const serviceId = `e2e-smoke-${suffix}`;
  const serviceName = 'UDDI E2E Smoke';
  const response = await fetch(`${apiUrl}/v1/admin/api-keys/`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Admin-Token': adminToken,
    },
    body: JSON.stringify({ serviceId, serviceName }),
  });
  const body = await readJSON(response);
  assert(response.ok, `Failed to create smoke API key: ${formatPayload(body)}`);
  assert(typeof body.apiKey === 'string', 'Admin API should return plaintext apiKey once');
  return {
    serviceId,
    serviceName,
    apiKey: body.apiKey,
  };
}

async function resolveDID(did) {
  const response = await fetch(`${apiUrl}/v1/did/${did}`);
  const body = await readJSON(response);
  assert(response.ok, `Failed to resolve DID: ${formatPayload(body)}`);
  return body;
}

async function expectOK(path) {
  const response = await fetch(`${apiUrl}${path}`);
  assert(response.ok, `${path} should return a successful response, got ${response.status}`);
}

async function waitForReady() {
  const deadline = Date.now() + 30_000;
  let lastError = null;

  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${apiUrl}/ready`);
      if (response.ok) {
        return;
      }
      lastError = new Error(`/ready returned ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await sleep(1000);
  }

  throw new Error(`UDDI API was not ready within 30s: ${lastError?.message ?? 'unknown error'}`);
}

function createMemoryStorage() {
  let storedIdentity = null;
  return {
    async save(identity) {
      storedIdentity = identity;
    },
    async load() {
      return storedIdentity;
    },
    async delete() {
      storedIdentity = null;
    },
  };
}

async function readJSON(response) {
  const text = await response.text();
  if (!text) {
    return {};
  }
  try {
    return JSON.parse(text);
  } catch {
    return { body: text };
  }
}

function formatPayload(payload) {
  return JSON.stringify(payload);
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function sleep(ms) {
  return new Promise(resolve => {
    setTimeout(resolve, ms);
  });
}
