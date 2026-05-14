/**
 * UDDI Identity Module
 * Handles DID generation, key management, and identity operations
 */

import * as ed from '@noble/ed25519';
import { sha256 } from '@noble/hashes/sha256';
import { base58 } from '@scure/base';
import type { Did, DidDocument, UddiIdentity, UddiIdentityPublic, VerificationMethod } from '../types';

// ─── Constants ────────────────────────────────────────────────────────────────

const DID_PREFIX = 'did:uddi:z' as const;
const W3C_DID_CONTEXT = 'https://www.w3.org/ns/did/v1';
const ED25519_CONTEXT = 'https://w3id.org/security/suites/ed25519-2020/v1';
const UDDI_CONTEXT = 'https://uddi.network/v1';

// ─── DID Generation ───────────────────────────────────────────────────────────

/**
 * Generate a new UDDI identity
 * Private key is generated using cryptographically secure randomness
 *
 * @returns A new UddiIdentity with keypair and DID
 * @example
 * const identity = await generateIdentity();
 * console.log(identity.did);
 * // → did:uddi:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK
 */
export async function generateIdentity(): Promise<UddiIdentity> {
  // Generate cryptographically secure Ed25519 keypair
  const privateKey = ed.utils.randomPrivateKey();
  const publicKey = await ed.getPublicKeyAsync(privateKey);

  const did = publicKeyToDid(publicKey);

  return {
    did,
    privateKey,
    publicKey,
    createdAt: new Date().toISOString(),
  };
}

/**
 * Derive a DID from an existing public key
 * DID = "did:uddi:z" + base58(sha256(publicKey))
 */
export function publicKeyToDid(publicKey: Uint8Array): Did {
  const hash = sha256(publicKey);
  const encoded = base58.encode(hash);
  return `${DID_PREFIX}${encoded}` as Did;
}

/**
 * Extract the public key hash from a DID
 */
export function didToPublicKeyHash(did: Did): Uint8Array {
  const encoded = did.replace(DID_PREFIX, '');
  return base58.decode(encoded);
}

// ─── DID Document ─────────────────────────────────────────────────────────────

/**
 * Create a W3C-compliant DID Document from an identity
 */
export function createDidDocument(identity: UddiIdentityPublic): DidDocument {
  const keyId = `${identity.did}#keys-1`;

  const verificationMethod: VerificationMethod = {
    id: keyId,
    type: 'Ed25519VerificationKey2020',
    controller: identity.did,
    publicKeyMultibase: `z${base58.encode(Buffer.from(identity.publicKey, 'base64'))}`,
  };

  return {
    '@context': [W3C_DID_CONTEXT, ED25519_CONTEXT, UDDI_CONTEXT],
    id: identity.did,
    verificationMethod: [verificationMethod],
    authentication: [keyId],
    assertionMethod: [keyId],
    created: identity.createdAt,
    updated: new Date().toISOString(),
    deactivated: false,
  };
}

// ─── Signing & Verification ───────────────────────────────────────────────────

/**
 * Sign a message with an identity's private key
 *
 * @param message - Raw bytes or string to sign
 * @param privateKey - Ed25519 private key
 * @returns base64-encoded signature
 */
export async function signMessage(
  message: Uint8Array | string,
  privateKey: Uint8Array,
): Promise<string> {
  const bytes = typeof message === 'string'
    ? new TextEncoder().encode(message)
    : message;

  const signature = await ed.signAsync(bytes, privateKey);
  return Buffer.from(signature).toString('base64');
}

/**
 * Verify a signature against a public key
 *
 * @param message - Original message
 * @param signatureBase64 - base64-encoded signature
 * @param publicKey - Ed25519 public key bytes
 * @returns true if signature is valid
 */
export async function verifySignature(
  message: Uint8Array | string,
  signatureBase64: string,
  publicKey: Uint8Array,
): Promise<boolean> {
  try {
    const bytes = typeof message === 'string'
      ? new TextEncoder().encode(message)
      : message;

    const signature = Buffer.from(signatureBase64, 'base64');
    return await ed.verifyAsync(signature, bytes, publicKey);
  } catch {
    return false;
  }
}

// ─── Auth Challenge ───────────────────────────────────────────────────────────

/**
 * Create a signed authentication presentation for a challenge
 * Used when logging into a service that supports UDDI
 *
 * @param challenge - The challenge received from the service
 * @param identity - The user's UDDI identity
 * @returns base64-encoded signed presentation
 */
export async function createAuthPresentation(
  challengeId: string,
  nonce: string,
  identity: UddiIdentity,
): Promise<string> {
  const timestamp = Date.now();

  // Message = challengeId + nonce + did + timestamp
  const message = `${challengeId}:${nonce}:${identity.did}:${timestamp}`;
  const signature = await signMessage(message, identity.privateKey);

  const presentation = {
    did: identity.did,
    challengeId,
    signature,
    timestamp,
  };

  return Buffer.from(JSON.stringify(presentation)).toString('base64');
}

// ─── Identity Export / Import ─────────────────────────────────────────────────

/**
 * Export identity public data (safe to share, no private key)
 */
export function exportPublicIdentity(identity: UddiIdentity): UddiIdentityPublic {
  return {
    did: identity.did,
    publicKey: Buffer.from(identity.publicKey).toString('base64'),
    createdAt: identity.createdAt,
  };
}

/**
 * Validate a DID format
 */
export function isValidDid(did: string): did is Did {
  return /^did:uddi:z[1-9A-HJ-NP-Za-km-z]{40,}$/.test(did);
}

/**
 * Check if two DIDs are equal (case-sensitive)
 */
export function didEquals(a: Did, b: Did): boolean {
  return a === b;
}
