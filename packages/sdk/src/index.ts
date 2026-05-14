/**
 * @uddi/sdk — Universal Decentralized Digital Identity SDK
 *
 * The main entry point for integrating UDDI into your application.
 *
 * @example Client (user-side)
 * ```typescript
 * import { UddiClient } from '@uddi/sdk';
 * const client = new UddiClient({ network: 'testnet' });
 * const identity = await client.createIdentity();
 * ```
 *
 * @example Verifier (app developer side)
 * ```typescript
 * import { UddiVerifier } from '@uddi/sdk';
 * const verifier = new UddiVerifier({ serviceId: 'my-app', apiKey: '...' });
 * const challenge = await verifier.createAuthChallenge();
 * ```
 */

import {
  generateIdentity,
  signMessage,
  createAuthPresentation,
  exportPublicIdentity,
  isValidDid,
} from '@uddi/core/identity';

import type {
  Did,
  UddiIdentity,
  NetworkConfig,
  NetworkType,
  AuthChallenge,
  AuthVerificationResult,
  VerifiableCredential,
  ProofVerificationResult,
  ZkProof,
  ClaimRequest,
} from '@uddi/core/types';

// ─── Network Configuration ────────────────────────────────────────────────────

const NETWORK_CONFIGS: Record<NetworkType, { apiUrl: string }> = {
  mainnet: { apiUrl: 'https://api.uddi.network' },
  testnet: { apiUrl: 'https://testnet-api.uddi.network' },
  local:   { apiUrl: 'http://localhost:8080' },
};

// ─── UddiClient ───────────────────────────────────────────────────────────────

export interface UddiClientConfig extends NetworkConfig {
  storage?: IdentityStorage;
}

/** Interface for storing identity (implement for your platform) */
export interface IdentityStorage {
  save(identity: UddiIdentity): Promise<void>;
  load(): Promise<UddiIdentity | null>;
  delete(): Promise<void>;
}

/**
 * UddiClient — used on the user/holder side
 *
 * Handles identity creation, credential storage, and authentication.
 * The private key NEVER leaves the device.
 */
export class UddiClient {
  private readonly apiUrl: string;
  private readonly storage: IdentityStorage | null;
  private identity: UddiIdentity | null = null;

  constructor(config: UddiClientConfig = { network: 'testnet' }) {
    const net = NETWORK_CONFIGS[config.network ?? 'testnet'];
    this.apiUrl = config.apiUrl ?? net.apiUrl;
    this.storage = config.storage ?? null;
  }

  /**
   * Create a new UDDI identity
   * Generates a keypair, derives DID, and registers it on the blockchain
   *
   * @returns The new DID (private key stays in memory / storage)
   */
  async createIdentity(): Promise<{ did: Did }> {
    if (this.identity) {
      throw new Error('Identity already exists. Call deleteIdentity() first.');
    }

    // Generate keypair locally
    this.identity = await generateIdentity();

    // Persist if storage is configured
    if (this.storage) {
      await this.storage.save(this.identity);
    }

    // Register DID on blockchain
    await this.registerOnBlockchain(this.identity);

    return { did: this.identity.did };
  }

  /**
   * Load existing identity from storage
   */
  async loadIdentity(): Promise<{ did: Did } | null> {
    if (!this.storage) {
      throw new Error('No storage configured. Pass storage in UddiClientConfig.');
    }

    this.identity = await this.storage.load();
    return this.identity ? { did: this.identity.did } : null;
  }

  /**
   * Get the current DID
   */
  getDid(): Did | null {
    return this.identity?.did ?? null;
  }

  /**
   * Authenticate to a service — replaces username/password
   *
   * @param challenge - The AuthChallenge received from the service
   * @returns base64-encoded signed presentation
   *
   * @example
   * const challenge = await fetch('/auth/challenge').then(r => r.json());
   * const presentation = await client.authenticate(challenge);
   * await fetch('/auth/login', { method: 'POST', body: JSON.stringify({ presentation }) });
   */
  async authenticate(challenge: AuthChallenge): Promise<string> {
    this.requireIdentity();

    return createAuthPresentation(
      challenge.challengeId,
      challenge.nonce,
      this.identity!,
    );
  }

  /**
   * Generate a Zero-Knowledge Proof for a specific claim
   * Proves something without revealing underlying data
   *
   * @example Prove you are 18+ without revealing birth date
   * const proof = await client.generateProof({ type: 'age', minimumAge: 18 });
   */
  async generateProof(request: {
    type: 'age' | 'citizenship' | 'kyc' | 'education';
    params?: Record<string, unknown>;
  }): Promise<ZkProof> {
    this.requireIdentity();

    const response = await this.post('/v1/proof/generate', {
      did: this.identity!.did,
      type: request.type,
      params: request.params ?? {},
    });

    return response.proof as ZkProof;
  }

  /**
   * Get all credentials stored in the UDDI registry for this DID
   */
  async getCredentials(): Promise<VerifiableCredential<object>[]> {
    this.requireIdentity();

    const response = await this.get(`/v1/credentials/${this.identity!.did}`);
    return response.credentials as VerifiableCredential<object>[];
  }

  /**
   * Revoke the current identity (cannot be undone)
   */
  async revokeIdentity(): Promise<void> {
    this.requireIdentity();

    const timestamp = Date.now().toString();
    const message = `revoke:${this.identity!.did}:${timestamp}`;
    const signature = await signMessage(message, this.identity!.privateKey);

    await this.post('/v1/did/revoke', {
      did: this.identity!.did,
      signatureBase64: signature,
      timestamp,
    });

    if (this.storage) {
      await this.storage.delete();
    }
    this.identity = null;
  }

  // ─── Private Methods ────────────────────────────────────────────────────────

  private async registerOnBlockchain(identity: UddiIdentity): Promise<void> {
    const timestamp = Date.now().toString();
    const message = `register:${identity.did}:${timestamp}`;
    const signature = await signMessage(message, identity.privateKey);
    const publicIdentity = exportPublicIdentity(identity);

    await this.post('/v1/did/register', {
      did: identity.did,
      publicKeyBase64: publicIdentity.publicKey,
      signatureBase64: signature,
      timestamp,
    });
  }

  private requireIdentity(): void {
    if (!this.identity) {
      throw new Error('No identity loaded. Call createIdentity() or loadIdentity() first.');
    }
  }

  private async get(path: string): Promise<Record<string, unknown>> {
    const res = await fetch(`${this.apiUrl}${path}`);
    if (!res.ok) throw new Error(`UDDI API error: ${res.status}`);
    return res.json();
  }

  private async post(path: string, body: unknown): Promise<Record<string, unknown>> {
    const res = await fetch(`${this.apiUrl}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const error = await readAPIError(res);
      throw new Error(`UDDI API error: ${error}`);
    }
    return res.json();
  }
}

// ─── UddiVerifier ─────────────────────────────────────────────────────────────

export interface UddiVerifierConfig extends NetworkConfig {
  serviceId: string;
  apiKey: string;
  serviceName?: string;
}

/**
 * UddiVerifier — used by app developers to verify UDDI identities
 *
 * Integrate UDDI login and claim verification into your application.
 *
 * @example
 * ```typescript
 * // Express.js example
 * const verifier = new UddiVerifier({
 *   serviceId: 'my-app',
 *   apiKey: process.env.UDDI_API_KEY,
 *   network: 'testnet',
 * });
 *
 * app.get('/auth/challenge', async (req, res) => {
 *   const challenge = await verifier.createAuthChallenge();
 *   res.json(challenge);
 * });
 *
 * app.post('/auth/verify', async (req, res) => {
 *   const result = await verifier.verifyAuth(req.body.challengeId, req.body.presentation);
 *   if (result.valid) res.json({ token: createJWT(result.did) });
 *   else res.status(401).json({ error: 'Invalid identity' });
 * });
 * ```
 */
export class UddiVerifier {
  private readonly apiUrl: string;
  private readonly serviceId: string;
  private readonly apiKey: string;
  private readonly serviceName: string;

  constructor(config: UddiVerifierConfig) {
    const net = NETWORK_CONFIGS[config.network ?? 'testnet'];
    this.apiUrl = config.apiUrl ?? net.apiUrl;
    this.serviceId = config.serviceId;
    this.apiKey = config.apiKey;
    this.serviceName = config.serviceName ?? config.serviceId;
  }

  /**
   * Create an authentication challenge
   * Present the QR code or deep link to the user
   *
   * @param requestedClaims - Optional claims to request from the user
   */
  async createAuthChallenge(requestedClaims?: ClaimRequest[]): Promise<AuthChallenge> {
    const response = await this.post('/v1/verify/challenge', {
      serviceId: this.serviceId,
      serviceName: this.serviceName,
      requestedClaims: requestedClaims ?? [],
    });

    return response as unknown as AuthChallenge;
  }

  /**
   * Verify a user's authentication presentation
   *
   * @param challengeId - The challenge ID from createAuthChallenge
   * @param presentationBase64 - The signed presentation from the user's app
   */
  async verifyAuth(
    challengeId: string,
    presentationBase64: string,
  ): Promise<AuthVerificationResult> {
    const response = await this.post('/v1/verify/auth', {
      serviceId: this.serviceId,
      challengeId,
      presentation: presentationBase64,
    });

    return response as unknown as AuthVerificationResult;
  }

  /**
   * Verify a specific claim via Zero-Knowledge Proof
   * The user's actual data is NEVER revealed
   *
   * @example Check if user is 18+ without seeing birth date
   * ```typescript
   * const result = await verifier.verifyClaim(did, {
   *   type: 'age',
   *   params: { minimumAge: 18 },
   *   proof: userZkProof,
   * });
   * console.log(result.valid); // true / false
   * ```
   */
  async verifyClaim(
    did: Did,
    claim: {
      type: 'age' | 'citizenship' | 'kyc' | 'education';
      params?: Record<string, unknown>;
      proof: ZkProof;
    },
  ): Promise<ProofVerificationResult> {
    if (!isValidDid(did)) {
      throw new Error(`Invalid DID format: ${did}`);
    }

    const response = await this.post('/v1/verify/claim', {
      did,
      claimType: claim.type,
      params: claim.params ?? {},
      proof: claim.proof,
    });

    return response as unknown as ProofVerificationResult;
  }

  /**
   * Resolve a DID to its DID Document
   */
  async resolveDid(did: Did): Promise<object> {
    if (!isValidDid(did)) throw new Error(`Invalid DID: ${did}`);

    const response = await this.get(`/v1/did/${did}`);
    return response;
  }

  // ─── Private Methods ──────────────────────────────────────────────────────

  private get authHeaders() {
    return {
      'Content-Type': 'application/json',
      'X-API-Key': this.apiKey,
      'X-Service-ID': this.serviceId,
    };
  }

  private async get(path: string): Promise<Record<string, unknown>> {
    const res = await fetch(`${this.apiUrl}${path}`, {
      headers: this.authHeaders,
    });
    if (!res.ok) throw new Error(`UDDI API error: ${res.status}`);
    return res.json();
  }

  private async post(path: string, body: unknown): Promise<Record<string, unknown>> {
    const res = await fetch(`${this.apiUrl}${path}`, {
      method: 'POST',
      headers: this.authHeaders,
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const error = await readAPIError(res);
      throw new Error(`UDDI API error: ${error}`);
    }
    return res.json();
  }
}

async function readAPIError(res: Response): Promise<string | number> {
  const error = await res.json().catch(() => null) as { message?: string; error?: string } | null;
  return error?.message ?? error?.error ?? res.statusText ?? res.status;
}

// ─── Re-exports ───────────────────────────────────────────────────────────────

export { generateIdentity, isValidDid } from '@uddi/core/identity';
export { issueCredential, verifyCredential } from '@uddi/core/vc';
export type {
  Did,
  UddiIdentity,
  VerifiableCredential,
  AuthChallenge,
  AuthVerificationResult,
  ZkProof,
  NetworkType,
} from '@uddi/core/types';
