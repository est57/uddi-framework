import { UddiClient, UddiVerifier } from '@uddi/sdk';

const apiUrl = process.env.UDDI_API_URL ?? 'http://localhost:8080';
const serviceId = process.env.UDDI_SERVICE_ID ?? 'dev-service';
const apiKey = process.env.UDDI_API_KEY ?? 'dev-api-key';
const serviceName = process.env.UDDI_SERVICE_NAME ?? 'UDDI Example Verifier';

async function main() {
  await assertAPIIsRunning();

  const holder = new UddiClient({
    network: 'local',
    apiUrl,
  });

  const verifier = new UddiVerifier({
    network: 'local',
    apiUrl,
    serviceId,
    apiKey,
    serviceName,
  });

  logStep('Creating and registering holder identity');
  const identity = await holder.createIdentity();
  console.log(`DID: ${identity.did}`);

  logStep('Resolving registered DID');
  const didResolution = await verifier.resolveDid(identity.did);
  console.log(`Resolved: ${Boolean(didResolution.didDocument)}`);

  logStep('Creating verifier challenge');
  const challenge = await verifier.createAuthChallenge();
  console.log(`Challenge: ${challenge.challengeId}`);

  logStep('Signing challenge from holder side');
  const presentation = await holder.authenticate(challenge);
  console.log(`Presentation bytes: ${Buffer.byteLength(presentation, 'base64')}`);

  logStep('Verifying authentication presentation');
  const authResult = await verifier.verifyAuth(challenge.challengeId, presentation);
  console.log(JSON.stringify(authResult, null, 2));

  if (!authResult.valid) {
    throw new Error(`Expected authentication to be valid: ${authResult.reason ?? 'unknown reason'}`);
  }

  logStep('Generating and verifying proof stub');
  const proof = await holder.generateProof({
    type: 'age',
    params: { minimumAge: 18 },
  });
  const claimResult = await verifier.verifyClaim(identity.did, {
    type: 'age',
    params: { minimumAge: 18 },
    proof,
  });
  console.log(JSON.stringify(claimResult, null, 2));

  if (!claimResult.valid) {
    throw new Error('Expected claim verification to be valid');
  }

  logStep('Auth flow completed');
}

async function assertAPIIsRunning() {
  let response;
  try {
    response = await fetch(`${apiUrl}/health`);
  } catch (error) {
    throw new Error(
      `UDDI API is not reachable at ${apiUrl}. Start it with: docker compose -f infra/docker/docker-compose.dev.yml up --build`,
      { cause: error },
    );
  }

  if (!response.ok) {
    throw new Error(`UDDI API health check failed with HTTP ${response.status}`);
  }
}

function logStep(message) {
  console.log(`\n> ${message}`);
}

main().catch((error) => {
  console.error('\nExample failed');
  console.error(error.message);
  process.exitCode = 1;
});
