/**
 * UDDI Core Types
 * All fundamental types used across the UDDI protocol
 */

// ─── DID Types ───────────────────────────────────────────────────────────────

/** A W3C-compliant Decentralized Identifier */
export type Did = `did:uddi:${string}`;

/** Supported verification method types */
export type VerificationMethodType = 'Ed25519VerificationKey2020' | 'JsonWebKey2020';

/** A cryptographic key associated with a DID */
export interface VerificationMethod {
  id: string;
  type: VerificationMethodType;
  controller: Did;
  publicKeyMultibase?: string;
  publicKeyJwk?: JsonWebKey;
}

/** W3C DID Document */
export interface DidDocument {
  '@context': string[];
  id: Did;
  controller?: Did | Did[];
  verificationMethod: VerificationMethod[];
  authentication: (string | VerificationMethod)[];
  assertionMethod?: (string | VerificationMethod)[];
  keyAgreement?: (string | VerificationMethod)[];
  created: string;   // ISO 8601
  updated: string;   // ISO 8601
  deactivated?: boolean;
}

/** Result of resolving a DID */
export interface DidResolutionResult {
  didDocument: DidDocument | null;
  didDocumentMetadata: {
    created: string;
    updated: string;
    deactivated: boolean;
    versionId: string;
  };
  didResolutionMetadata: {
    contentType: string;
    error?: string;
  };
}

// ─── Identity Types ───────────────────────────────────────────────────────────

/** A UDDI identity stored on the user's device */
export interface UddiIdentity {
  did: Did;
  privateKey: Uint8Array;   // NEVER leaves device
  publicKey: Uint8Array;
  createdAt: string;        // ISO 8601
}

/** Exported identity (no private key) safe for sharing */
export interface UddiIdentityPublic {
  did: Did;
  publicKey: string;        // base64
  createdAt: string;
}

// ─── Verifiable Credential Types ─────────────────────────────────────────────

/** Supported credential types */
export type CredentialType =
  | 'AgeCredential'
  | 'CitizenshipCredential'
  | 'KycCredential'
  | 'EducationCredential'
  | 'HealthCredential'
  | 'ProfessionalCredential';

/** A W3C Verifiable Credential */
export interface VerifiableCredential<T extends object = Record<string, unknown>> {
  '@context': string[];
  id: string;
  type: ['VerifiableCredential', ...string[]];
  issuer: Did | { id: Did; name: string };
  issuanceDate: string;
  expirationDate?: string;
  credentialSubject: {
    id: Did;
  } & T;
  proof: CredentialProof;
}

/** Cryptographic proof on a credential */
export interface CredentialProof {
  type: string;
  created: string;
  verificationMethod: string;
  proofPurpose: 'assertionMethod' | 'authentication';
  proofValue: string;   // base64url encoded signature
}

/** Age credential subject */
export interface AgeCredentialSubject {
  id: Did;
  birthYear: number;
  birthMonth: number;
  birthDay: number;
  issuingCountry: string;
}

/** KYC credential subject */
export interface KycCredentialSubject {
  id: Did;
  level: 'basic' | 'enhanced' | 'full';
  verifiedAt: string;
  issuingInstitution: string;
}

// ─── ZKP Types ───────────────────────────────────────────────────────────────

/** A Zero-Knowledge Proof */
export interface ZkProof {
  protocol: 'groth16' | 'plonk';
  curve: 'bn128' | 'bls12381';
  proof: {
    pi_a: string[];
    pi_b: string[][];
    pi_c: string[];
  };
  publicSignals: string[];
}

/** Request to generate a ZK proof */
export interface ProofRequest {
  type: 'age' | 'citizenship' | 'kyc' | 'education';
  did: Did;
  params: Record<string, unknown>;
  credentialId: string;
}

/** Result of a ZKP verification */
export interface ProofVerificationResult {
  valid: boolean;
  claimType: string;
  verifiedAt: string;
  publicClaims: Record<string, unknown>;  // ONLY what was requested, no raw data
}

// ─── Network Types ────────────────────────────────────────────────────────────

export type NetworkType = 'mainnet' | 'testnet' | 'local';

export interface NetworkConfig {
  network: NetworkType;
  apiUrl?: string;
  blockchainRpc?: string;
  zkpServiceUrl?: string;
}

// ─── Auth Types ───────────────────────────────────────────────────────────────

/** An authentication challenge issued by a service */
export interface AuthChallenge {
  challengeId: string;
  nonce: string;
  serviceId: string;
  serviceName: string;
  issuedAt: string;
  expiresAt: string;
  requestedClaims?: ClaimRequest[];
  qrCode?: string;
}

/** A claim requested by a verifier */
export interface ClaimRequest {
  type: CredentialType;
  required: boolean;
  params?: Record<string, unknown>;
}

/** A signed authentication presentation from a user */
export interface AuthPresentation {
  did: Did;
  challengeId: string;
  signature: string;     // base64
  timestamp: number;
  proofs?: ZkProof[];    // optional ZK proofs for claims
}

/** Result of authentication verification */
export interface AuthVerificationResult {
  valid: boolean;
  did: Did;
  verifiedAt: string;
  verifiedClaims: ProofVerificationResult[];
}
