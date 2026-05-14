/**
 * UDDI SDK — Complete Integration Examples
 * ==========================================
 * This file shows real-world usage of the UDDI SDK
 * for app developers wanting to integrate UDDI.
 */

import { UddiClient, UddiVerifier } from '@uddi/sdk';

// =============================================================================
// EXAMPLE 1: User Creates a New Digital Identity
// (runs on user's device / mobile app)
// =============================================================================

async function example1_createIdentity() {
  console.log('=== Example 1: Create Identity ===\n');

  const client = new UddiClient({ network: 'testnet' });

  // Generate keypair & register DID on blockchain
  const { did } = await client.createIdentity();

  console.log('✅ Identity created!');
  console.log('DID:', did);
  // → did:uddi:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK

  // Private key NEVER leaves the device
  // DID is now registered on the UDDI blockchain
}

// =============================================================================
// EXAMPLE 2: Login to a Website with UDDI (no password!)
// =============================================================================

// --- On the website (server side) ---
async function example2_server_createChallenge() {
  const verifier = new UddiVerifier({
    network: 'testnet',
    serviceId: 'my-ecommerce-site',
    apiKey: process.env.UDDI_API_KEY!,
    serviceName: 'My E-Commerce Store',
  });

  // Create a login challenge
  const challenge = await verifier.createAuthChallenge();

  console.log('Challenge created:', challenge.challengeId);
  console.log('Show this QR code to the user:', challenge.qrCode);
  // → User scans QR with UDDI mobile app

  return challenge;
}

// --- On the user's device (client side) ---
async function example2_client_authenticate(challenge: any) {
  const client = new UddiClient({ network: 'testnet' });
  await client.loadIdentity(); // Load from device storage

  // Sign the challenge — proves identity without password
  const presentation = await client.authenticate(challenge);
  console.log('Signed presentation created ✅');

  return presentation;
}

// --- Back on the server: verify the user ---
async function example2_server_verify(challengeId: string, presentation: string) {
  const verifier = new UddiVerifier({
    network: 'testnet',
    serviceId: 'my-ecommerce-site',
    apiKey: process.env.UDDI_API_KEY!,
  });

  const result = await verifier.verifyAuth(challengeId, presentation);

  if (result.valid) {
    console.log('✅ User authenticated!');
    console.log('User DID:', result.did);
    // Use result.did as the user's permanent identifier
    // Create a session / JWT token for your app
  } else {
    console.log('❌ Authentication failed');
  }
}

// =============================================================================
// EXAMPLE 3: Verify Age (18+) Without Seeing Birth Date
// Zero-Knowledge Proof in action!
// =============================================================================

// --- User generates ZKP on their device ---
async function example3_generateAgeProof() {
  const client = new UddiClient({ network: 'testnet' });
  await client.loadIdentity();

  // Generate proof: "I am 18 or older"
  // Birth date is NEVER sent anywhere
  const proof = await client.generateProof({
    type: 'age',
    params: { minimumAge: 18 },
  });

  console.log('ZKP generated ✅');
  console.log('Proof size:', JSON.stringify(proof).length, 'bytes');

  return proof;
}

// --- Verifier checks the proof ---
async function example3_verifyAge(did: string, proof: any) {
  const verifier = new UddiVerifier({
    network: 'testnet',
    serviceId: 'alcohol-shop',
    apiKey: process.env.UDDI_API_KEY!,
  });

  const result = await verifier.verifyClaim(did as any, {
    type: 'age',
    params: { minimumAge: 18 },
    proof,
  });

  if (result.valid) {
    console.log('✅ User is 18 or older — access granted');
    console.log('Verified at:', result.verifiedAt);
    // The verifier knows ONLY: "this user is 18+"
    // The verifier does NOT know the actual birth date
  } else {
    console.log('❌ Age verification failed');
  }
}

// =============================================================================
// EXAMPLE 4: Express.js Integration (Full Flow)
// =============================================================================

import express from 'express';

function createUddiExpressApp() {
  const app = express();
  app.use(express.json());

  const verifier = new UddiVerifier({
    network: 'testnet',
    serviceId: 'my-app',
    apiKey: process.env.UDDI_API_KEY!,
    serviceName: 'My Application',
  });

  // Store challenges temporarily (use Redis in production)
  const challenges = new Map<string, any>();

  // Step 1: Generate login challenge
  app.get('/auth/uddi/challenge', async (req, res) => {
    try {
      const challenge = await verifier.createAuthChallenge();
      challenges.set(challenge.challengeId, challenge);

      // Auto-expire after 5 minutes
      setTimeout(() => challenges.delete(challenge.challengeId), 5 * 60 * 1000);

      res.json({
        challengeId: challenge.challengeId,
        qrCode: challenge.qrCode,
        expiresAt: challenge.expiresAt,
      });
    } catch (err) {
      res.status(500).json({ error: 'Failed to create challenge' });
    }
  });

  // Step 2: Verify user's signed presentation
  app.post('/auth/uddi/verify', async (req, res) => {
    const { challengeId, presentation } = req.body;

    if (!challenges.has(challengeId)) {
      return res.status(400).json({ error: 'Invalid or expired challenge' });
    }

    try {
      const result = await verifier.verifyAuth(challengeId, presentation);
      challenges.delete(challengeId);

      if (result.valid) {
        // Create session (use your preferred JWT library)
        const token = Buffer.from(JSON.stringify({
          did: result.did,
          iat: Date.now(),
          exp: Date.now() + 24 * 60 * 60 * 1000, // 24h
        })).toString('base64');

        res.json({ token, did: result.did });
      } else {
        res.status(401).json({ error: 'Authentication failed' });
      }
    } catch (err) {
      res.status(500).json({ error: 'Verification failed' });
    }
  });

  // Step 3 (optional): Protected route that requires age verification
  app.get('/protected/adults-only', async (req, res) => {
    const did = req.headers['x-uddi-did'] as string;
    const proof = req.headers['x-uddi-age-proof'];

    if (!did || !proof) {
      return res.status(401).json({ error: 'UDDI identity required' });
    }

    const ageResult = await verifier.verifyClaim(did as any, {
      type: 'age',
      params: { minimumAge: 18 },
      proof: JSON.parse(Buffer.from(proof as string, 'base64').toString()),
    });

    if (!ageResult.valid) {
      return res.status(403).json({ error: 'Must be 18+ to access this content' });
    }

    res.json({ message: 'Welcome!', verifiedAge: '18+' });
  });

  return app;
}

// =============================================================================
// Run examples
// =============================================================================

(async () => {
  console.log('UDDI SDK Examples\n');
  await example1_createIdentity();
})();
