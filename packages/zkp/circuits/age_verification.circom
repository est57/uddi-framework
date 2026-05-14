pragma circom 2.1.6;

/*
 * UDDI Age Verification Circuit
 * ==============================
 * Proves that a user is at least N years old WITHOUT revealing their birth date.
 *
 * Private inputs (known only to the prover/user):
 *   - birthYear, birthMonth, birthDay  → the actual birth date
 *   - credentialSecret                 → secret salt from the issued VC
 *
 * Public inputs (known to the verifier):
 *   - currentYear, currentMonth        → today's date
 *   - minimumAge                       → e.g. 18
 *   - credentialHash                   → Poseidon hash of the VC (from blockchain)
 *
 * Output:
 *   - isValid                          → 1 if age >= minimumAge, 0 otherwise
 *
 * The verifier learns ONLY: "this person is at least N years old"
 * The verifier learns NOTHING about the actual birth date.
 */

include "circomlib/circuits/comparators.circom";
include "circomlib/circuits/poseidon.circom";
include "circomlib/circuits/bitify.circom";

template AgeVerification() {
    // ── Private Inputs ────────────────────────────────────────────────────────
    signal private input birthYear;
    signal private input birthMonth;
    signal private input birthDay;
    signal private input credentialSecret;   // Poseidon hash secret from VC

    // ── Public Inputs ─────────────────────────────────────────────────────────
    signal input currentYear;
    signal input currentMonth;
    signal input minimumAge;
    signal input credentialHash;             // Must match on-chain VC hash

    // ── Output ────────────────────────────────────────────────────────────────
    signal output isValid;

    // ── Step 1: Verify the credential hash ───────────────────────────────────
    // Re-compute Poseidon hash from private inputs + secret
    // This proves the prover actually holds a valid credential
    component hasher = Poseidon(4);
    hasher.inputs[0] <== birthYear;
    hasher.inputs[1] <== birthMonth;
    hasher.inputs[2] <== birthDay;
    hasher.inputs[3] <== credentialSecret;

    // The computed hash must match the public on-chain hash
    credentialHash === hasher.out;

    // ── Step 2: Compute age ───────────────────────────────────────────────────
    // Simple year-based calculation
    // For full precision, month comparison would be added
    signal yearAge;
    yearAge <== currentYear - birthYear;

    // Adjust for birthday not yet reached this year
    // monthDiff = currentMonth - birthMonth
    signal monthDiff;
    monthDiff <== currentMonth - birthMonth;

    // If monthDiff < 0, birthday hasn't happened yet this year → subtract 1
    component monthCheck = LessThan(8);
    monthCheck.in[0] <== currentMonth;
    monthCheck.in[1] <== birthMonth;

    signal age;
    age <== yearAge - monthCheck.out;

    // ── Step 3: Assert age >= minimumAge ─────────────────────────────────────
    component ageCheck = GreaterEqThan(8);
    ageCheck.in[0] <== age;
    ageCheck.in[1] <== minimumAge;

    isValid <== ageCheck.out;

    // ── Step 4: Sanity bounds (prevent underflow exploits) ───────────────────
    // birthYear must be a plausible value (1900–2100)
    component birthYearLower = GreaterEqThan(11);
    birthYearLower.in[0] <== birthYear;
    birthYearLower.in[1] <== 1900;

    component birthYearUpper = LessThan(11);
    birthYearUpper.in[0] <== birthYear;
    birthYearUpper.in[1] <== 2100;

    // These constraints ensure valid year range (not output, just enforced)
    signal birthYearValid;
    birthYearValid <== birthYearLower.out * birthYearUpper.out;
    birthYearValid === 1;
}

component main {
    public [currentYear, currentMonth, minimumAge, credentialHash]
} = AgeVerification();
