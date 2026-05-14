/**
 * UDDI Verifiable Credentials Module
 * Issue, verify, and manage W3C Verifiable Credentials
 */

import { sha256 } from '@noble/hashes/sha256';
import { base58 } from '@scure/base';
import { signMessage, verifySignature } from '../identity';
import type {
  Did,
  VerifiableCredential,
  CredentialProof,
  CredentialType,
  UddiIdentity,
  AgeCredentialSubject,
  KycCredentialSubject,
} from '../types';

// ─── Constants ────────────────────────────────────────────────────────────────

const VC_CONTEXT = [
  'https://www.w3.org/2018/credentials/v1',
  'https://uddi.network/credentials/v1',
];

// ─── Credential Issuance ──────────────────────────────────────────────────────

/**
 * Issue a new Verifiable Credential
 * Called by trusted institutions (governments, universities, banks)
 *
 * @param issuerIdentity - Identity of the issuing institution
 * @param subjectDid - DID of the credential recipient
 * @param type - Type of credential
 * @param claims - The credential claims/data
 * @param expiresInDays - Optional expiration
 */
export async function issueCredential<T extends object>(
  issuerIdentity: UddiIdentity,
  subjectDid: Did,
  type: CredentialType,
  claims: T,
  expiresInDays?: number,
): Promise<VerifiableCredential<T>> {
  const issuanceDate = new Date().toISOString();
  const credentialId = generateCredentialId(issuerIdentity.did, subjectDid, type);

  const expirationDate = expiresInDays
    ? new Date(Date.now() + expiresInDays * 86_400_000).toISOString()
    : undefined;

  const credential = {
    '@context': VC_CONTEXT,
    id: credentialId,
    type: ['VerifiableCredential', type],
    issuer: issuerIdentity.did,
    issuanceDate,
    ...(expirationDate && { expirationDate }),
    credentialSubject: {
      id: subjectDid,
      ...claims,
    },
  } satisfies Omit<VerifiableCredential<T>, 'proof'>;

  // Sign the credential
  const proof = await createCredentialProof(credential, issuerIdentity);

  return { ...credential, proof };
}

/**
 * Issue an Age Credential (issued by government)
 */
export async function issueAgeCredential(
  issuerIdentity: UddiIdentity,
  subjectDid: Did,
  claims: Omit<AgeCredentialSubject, 'id'>,
): Promise<VerifiableCredential<Omit<AgeCredentialSubject, 'id'>>> {
  return issueCredential(
    issuerIdentity,
    subjectDid,
    'AgeCredential',
    claims,
    365 * 10, // 10 year validity
  );
}

/**
 * Issue a KYC Credential (issued by financial institution)
 */
export async function issueKycCredential(
  issuerIdentity: UddiIdentity,
  subjectDid: Did,
  claims: Omit<KycCredentialSubject, 'id'>,
): Promise<VerifiableCredential<Omit<KycCredentialSubject, 'id'>>> {
  return issueCredential(
    issuerIdentity,
    subjectDid,
    'KycCredential',
    claims,
    365, // 1 year validity
  );
}

// ─── Credential Verification ──────────────────────────────────────────────────

/**
 * Verify a Verifiable Credential's authenticity
 * Checks signature, expiration, and format
 *
 * @returns true if credential is valid and not expired
 */
export async function verifyCredential<T extends object>(
  credential: VerifiableCredential<T>,
  issuerPublicKey: Uint8Array,
): Promise<{ valid: boolean; reason?: string }> {
  // 1. Check expiration
  if (credential.expirationDate) {
    const expired = new Date(credential.expirationDate) < new Date();
    if (expired) {
      return { valid: false, reason: 'Credential has expired' };
    }
  }

  // 2. Check required fields
  if (!credential.id || !credential.issuer || !credential.credentialSubject) {
    return { valid: false, reason: 'Missing required credential fields' };
  }

  // 3. Verify cryptographic proof
  const { proof } = credential;
  const credentialWithoutProof = withoutProof(credential);
  const message = canonicalize(credentialWithoutProof);
  const valid = await verifySignature(message, proof.proofValue, issuerPublicKey);

  if (!valid) {
    return { valid: false, reason: 'Invalid cryptographic proof' };
  }

  return { valid: true };
}

/**
 * Compute a stable hash of a credential (used as credentialHash in ZKP)
 */
export function hashCredential<T extends object>(credential: VerifiableCredential<T>): string {
  const credentialWithoutProof = withoutProof(credential);
  const canonical = canonicalize(credentialWithoutProof);
  const hash = sha256(new TextEncoder().encode(canonical));
  return Buffer.from(hash).toString('hex');
}

// ─── Credential Presentation ──────────────────────────────────────────────────

/**
 * Create a Verifiable Presentation (wraps one or more VCs for sharing)
 */
export async function createPresentation(
  credentials: VerifiableCredential<object>[],
  holderIdentity: UddiIdentity,
  challengeId?: string,
): Promise<object> {
  const presentation = {
    '@context': ['https://www.w3.org/2018/credentials/v1'],
    type: ['VerifiablePresentation'],
    holder: holderIdentity.did,
    verifiableCredential: credentials,
    ...(challengeId && { challenge: challengeId }),
    created: new Date().toISOString(),
  };

  const proof = await createCredentialProof(presentation, holderIdentity);
  return { ...presentation, proof };
}

// ─── Internal Helpers ─────────────────────────────────────────────────────────

async function createCredentialProof(
  data: object,
  issuerIdentity: UddiIdentity,
): Promise<CredentialProof> {
  const message = canonicalize(data);
  const proofValue = await signMessage(message, issuerIdentity.privateKey);

  return {
    type: 'Ed25519Signature2020',
    created: new Date().toISOString(),
    verificationMethod: `${issuerIdentity.did}#keys-1`,
    proofPurpose: 'assertionMethod',
    proofValue,
  };
}

function generateCredentialId(issuerDid: Did, subjectDid: Did, type: string): string {
  const input = `${issuerDid}:${subjectDid}:${type}:${Date.now()}`;
  const hash = sha256(new TextEncoder().encode(input));
  return `urn:uddi:vc:${base58.encode(hash)}`;
}

function withoutProof<T extends object>(
  credential: VerifiableCredential<T>,
): Omit<VerifiableCredential<T>, 'proof'> {
  const copy = { ...credential };
  delete (copy as Partial<VerifiableCredential<T>>).proof;
  return copy;
}

/**
 * Deterministic JSON serialization for signing
 * Keys are sorted alphabetically to ensure consistent output
 */
function canonicalize(obj: unknown): string {
  if (typeof obj !== 'object' || obj === null) {
    return JSON.stringify(obj);
  }

  if (Array.isArray(obj)) {
    return `[${obj.map(canonicalize).join(',')}]`;
  }

  const sortedKeys = Object.keys(obj as Record<string, unknown>).sort();
  const pairs = sortedKeys.map(key => {
    const value = (obj as Record<string, unknown>)[key];
    return `${JSON.stringify(key)}:${canonicalize(value)}`;
  });

  return `{${pairs.join(',')}}`;
}
