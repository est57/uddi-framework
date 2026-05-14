import { describe, expect, it } from 'vitest';
import { generateIdentity } from '../identity';
import {
  createPresentation,
  hashCredential,
  issueAgeCredential,
  issueCredential,
  verifyCredential,
} from './index';

describe('verifiable credentials', () => {
  it('issues and verifies a generic credential', async () => {
    const issuer = await generateIdentity();
    const holder = await generateIdentity();

    const credential = await issueCredential(
      issuer,
      holder.did,
      'EducationCredential',
      { degree: 'BSc Computer Science', institution: 'UDDI University' },
    );

    expect(credential.issuer).toBe(issuer.did);
    expect(credential.credentialSubject.id).toBe(holder.did);
    expect(credential.proof.verificationMethod).toBe(`${issuer.did}#keys-1`);

    await expect(verifyCredential(credential, issuer.publicKey)).resolves.toEqual({ valid: true });
  });

  it('rejects tampered credentials', async () => {
    const issuer = await generateIdentity();
    const holder = await generateIdentity();
    const credential = await issueCredential(issuer, holder.did, 'ProfessionalCredential', {
      role: 'Engineer',
    });

    const tampered = {
      ...credential,
      credentialSubject: {
        ...credential.credentialSubject,
        role: 'Admin',
      },
    };

    await expect(verifyCredential(tampered, issuer.publicKey)).resolves.toEqual({
      valid: false,
      reason: 'Invalid cryptographic proof',
    });
  });

  it('rejects expired credentials', async () => {
    const issuer = await generateIdentity();
    const holder = await generateIdentity();
    const credential = await issueCredential(
      issuer,
      holder.did,
      'KycCredential',
      { level: 'basic' as const, verifiedAt: new Date().toISOString(), issuingInstitution: 'Bank' },
      -1,
    );

    await expect(verifyCredential(credential, issuer.publicKey)).resolves.toEqual({
      valid: false,
      reason: 'Credential has expired',
    });
  });

  it('issues age credentials and hashes credentials deterministically', async () => {
    const issuer = await generateIdentity();
    const holder = await generateIdentity();
    const credential = await issueAgeCredential(issuer, holder.did, {
      birthYear: 1990,
      birthMonth: 1,
      birthDay: 1,
      issuingCountry: 'ID',
    });

    expect(credential.type).toEqual(['VerifiableCredential', 'AgeCredential']);
    expect(hashCredential(credential)).toMatch(/^[a-f0-9]{64}$/);
    expect(hashCredential(credential)).toBe(hashCredential(credential));
  });

  it('creates a signed presentation', async () => {
    const issuer = await generateIdentity();
    const holder = await generateIdentity();
    const credential = await issueCredential(issuer, holder.did, 'EducationCredential', {
      degree: 'BSc',
    });

    const presentation = await createPresentation([credential], holder, 'challenge-1');

    expect(presentation).toMatchObject({
      holder: holder.did,
      challenge: 'challenge-1',
    });
    expect(presentation).toHaveProperty('proof');
  });
});
