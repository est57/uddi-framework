import {
  createDidDocument,
  exportPublicIdentity,
  generateIdentity,
  signMessage,
  verifySignature,
} from '@uddi/core';
import { UddiClient, UddiVerifier } from '@uddi/sdk';

const identity = await generateIdentity();
const publicIdentity = exportPublicIdentity(identity);
const didDocument = createDidDocument(publicIdentity);

const message = 'hello from UDDI npm quickstart';
const signature = await signMessage(message, identity.privateKey);
const valid = await verifySignature(message, signature, identity.publicKey);

console.log('DID:', identity.did);
console.log('Signature valid:', valid);
console.log('DID document id:', didDocument.id);
console.log('SDK exports:', typeof UddiClient, typeof UddiVerifier);

if (!valid) {
  throw new Error('Expected signature to be valid');
}
if (didDocument.id !== identity.did) {
  throw new Error('Expected DID document id to match generated DID');
}
