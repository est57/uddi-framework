import { describe, expect, it } from 'vitest';
import {
  createDidDocument,
  exportPublicIdentity,
  generateIdentity,
  verifySignature,
} from './identity';
import {
  createPresentation,
  issueCredential,
  verifyCredential,
} from './vc';
import type { VerifiableCredential } from './types';

describe('DID and VC compliance baseline', () => {
  it('creates DID documents with required W3C DID Core fields', async () => {
    const identity = await generateIdentity();
    const didDocument = createDidDocument(exportPublicIdentity(identity));
    const verificationMethod = didDocument.verificationMethod[0];

    expect(didDocument['@context']).toEqual(expect.arrayContaining([
      'https://www.w3.org/ns/did/v1',
    ]));
    expect(didDocument.id).toBe(identity.did);
    expect(didDocument.authentication).toContain(`${identity.did}#keys-1`);
    expect(didDocument.assertionMethod).toContain(`${identity.did}#keys-1`);
    expect(didDocument.deactivated).toBe(false);

    expect(verificationMethod).toMatchObject({
      id: `${identity.did}#keys-1`,
      type: 'Ed25519VerificationKey2020',
      controller: identity.did,
    });
    expect(verificationMethod?.publicKeyMultibase).toMatch(/^z/);
  });

  it('issues verifiable credentials with required W3C VC fields', async () => {
    const issuer = await generateIdentity();
    const holder = await generateIdentity();
    const credential = await issueCredential(issuer, holder.did, 'EducationCredential', {
      degree: 'BSc Computer Science',
    });

    expect(credential['@context']).toEqual(expect.arrayContaining([
      'https://www.w3.org/2018/credentials/v1',
    ]));
    expect(credential.id).toMatch(/^urn:uddi:vc:/);
    expect(credential.type).toEqual(['VerifiableCredential', 'EducationCredential']);
    expect(credential.issuer).toBe(issuer.did);
    expect(new Date(credential.issuanceDate).toString()).not.toBe('Invalid Date');
    expect(credential.credentialSubject).toMatchObject({
      id: holder.did,
      degree: 'BSc Computer Science',
    });
    expect(credential.proof).toMatchObject({
      type: 'Ed25519Signature2020',
      verificationMethod: `${issuer.did}#keys-1`,
      proofPurpose: 'assertionMethod',
    });
    expect(typeof credential.proof.proofValue).toBe('string');
  });

  it('creates verifiable presentations with holder binding and challenge support', async () => {
    const issuer = await generateIdentity();
    const holder = await generateIdentity();
    const credential = await issueCredential(issuer, holder.did, 'ProfessionalCredential', {
      role: 'Engineer',
    });

    const presentation = await createPresentation([credential], holder, 'challenge-123') as {
      '@context': string[];
      type: string[];
      holder: string;
      challenge: string;
      verifiableCredential: unknown[];
      proof: {
        proofValue: string;
        verificationMethod: string;
      };
      [key: string]: unknown;
    };

    expect(presentation['@context']).toContain('https://www.w3.org/2018/credentials/v1');
    expect(presentation.type).toEqual(['VerifiablePresentation']);
    expect(presentation.holder).toBe(holder.did);
    expect(presentation.challenge).toBe('challenge-123');
    expect(presentation.verifiableCredential).toHaveLength(1);
    expect(presentation.proof.verificationMethod).toBe(`${holder.did}#keys-1`);
    expect(typeof presentation.proof.proofValue).toBe('string');
  });

  it('rejects credentials with missing required VC fields', async () => {
    const issuer = await generateIdentity();
    const holder = await generateIdentity();
    const credential = await issueCredential(issuer, holder.did, 'KycCredential', {
      level: 'basic' as const,
      verifiedAt: new Date().toISOString(),
      issuingInstitution: 'UDDI Bank',
    });

    const missingSubject = {
      ...credential,
      credentialSubject: undefined,
    } as unknown as VerifiableCredential;

    await expect(verifyCredential(missingSubject, issuer.publicKey)).resolves.toEqual({
      valid: false,
      reason: 'Missing required credential fields',
    });
  });

  it('rejects credentials verified against the wrong issuer key', async () => {
    const issuer = await generateIdentity();
    const otherIssuer = await generateIdentity();
    const holder = await generateIdentity();
    const credential = await issueCredential(issuer, holder.did, 'HealthCredential', {
      status: 'verified',
    });

    await expect(verifyCredential(credential, otherIssuer.publicKey)).resolves.toEqual({
      valid: false,
      reason: 'Invalid cryptographic proof',
    });
  });

  it('produces proofs that verify over canonicalized credential content only', async () => {
    const issuer = await generateIdentity();
    const holder = await generateIdentity();
    const credential = await issueCredential(issuer, holder.did, 'EducationCredential', {
      degree: 'MSc',
    });

    const message = canonicalizeForTest(withoutProofForTest(credential));
    await expect(verifySignature(
      message,
      credential.proof.proofValue,
      issuer.publicKey,
    )).resolves.toBe(true);
  });
});

function withoutProofForTest<T extends object>(
  credential: VerifiableCredential<T>,
): Omit<VerifiableCredential<T>, 'proof'> {
  const copy = { ...credential };
  delete (copy as Partial<VerifiableCredential<T>>).proof;
  return copy;
}

function canonicalizeForTest(obj: unknown): string {
  if (typeof obj !== 'object' || obj === null) {
    return JSON.stringify(obj);
  }

  if (Array.isArray(obj)) {
    return `[${obj.map(canonicalizeForTest).join(',')}]`;
  }

  const sortedKeys = Object.keys(obj as Record<string, unknown>).sort();
  const pairs = sortedKeys.map(key => {
    const value = (obj as Record<string, unknown>)[key];
    return `${JSON.stringify(key)}:${canonicalizeForTest(value)}`;
  });

  return `{${pairs.join(',')}}`;
}
