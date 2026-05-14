# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | ✅ Yes (alpha)     |
| < 0.1   | ❌ No              |

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities via public GitHub Issues.**

Since UDDI is a cryptographic identity protocol, security is our highest priority.

### How to Report

1. **Email**: Send details to `security@uddi.network`
2. **PGP**: Encrypt your message using our [PGP key](https://uddi.network/pgp-key.asc)
3. **Subject**: `[SECURITY] Brief description`

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

| Action | Timeframe |
|--------|-----------|
| Acknowledgement | Within 48 hours |
| Initial assessment | Within 5 days |
| Fix development | Depends on severity |
| Public disclosure | After fix is released |

### Severity Classification

| Severity | Examples |
|----------|---------|
| **Critical** | Private key exposure, signature forgery, ZKP soundness break |
| **High** | DID spoofing, credential forgery, authentication bypass |
| **Medium** | DoS on API, information leakage, weak randomness |
| **Low** | Minor information disclosure, non-critical bugs |

### Safe Harbor

We consider security research done in good faith to be authorized.
We will not take legal action against researchers who:
- Report vulnerabilities responsibly
- Do not access user data
- Do not disrupt services
- Give us reasonable time to fix before public disclosure

Thank you for helping keep UDDI safe. 🔐
