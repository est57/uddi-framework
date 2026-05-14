pragma circom 2.1.6;

/*
 * UDDI Citizenship Verification Circuit
 * =======================================
 * Proves that a user is a citizen of a specific country
 * WITHOUT revealing their national ID number or full identity.
 *
 * Private inputs:
 *   - nationalIdHash    → Poseidon hash of the national ID number
 *   - countryCode       → numeric country code (e.g. 360 = Indonesia)
 *   - credentialSecret  → secret salt from the issued VC
 *
 * Public inputs:
 *   - expectedCountry   → the country the verifier wants to confirm
 *   - credentialHash    → on-chain VC hash
 *
 * Output:
 *   - isCitizen         → 1 if citizen of expectedCountry, 0 otherwise
 */

include "circomlib/circuits/poseidon.circom";
include "circomlib/circuits/comparators.circom";

template CitizenshipVerification() {
    // ── Private Inputs ────────────────────────────────────────────────────────
    signal private input nationalIdHash;
    signal private input countryCode;       // ISO 3166-1 numeric
    signal private input credentialSecret;

    // ── Public Inputs ─────────────────────────────────────────────────────────
    signal input expectedCountry;
    signal input credentialHash;

    // ── Output ────────────────────────────────────────────────────────────────
    signal output isCitizen;

    // ── Step 1: Verify credential hash ───────────────────────────────────────
    component hasher = Poseidon(3);
    hasher.inputs[0] <== nationalIdHash;
    hasher.inputs[1] <== countryCode;
    hasher.inputs[2] <== credentialSecret;

    credentialHash === hasher.out;

    // ── Step 2: Check country matches ─────────────────────────────────────────
    component countryCheck = IsEqual();
    countryCheck.in[0] <== countryCode;
    countryCheck.in[1] <== expectedCountry;

    isCitizen <== countryCheck.out;
}

component main {
    public [expectedCountry, credentialHash]
} = CitizenshipVerification();
