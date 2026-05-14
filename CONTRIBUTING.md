# Contributing to UDDI

First off, **thank you** for considering contributing to UDDI! 🎉

UDDI is a community-driven open protocol. Every contribution — code, documentation, bug reports, ideas — helps build the future of digital identity.

---

## 📋 Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How to Contribute](#how-to-contribute)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Submitting a Pull Request](#submitting-a-pull-request)
- [Coding Standards](#coding-standards)
- [Commit Convention](#commit-convention)

---

## Code of Conduct

This project follows the [Contributor Covenant](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).
By participating, you agree to uphold a respectful and inclusive environment.

---

## How to Contribute

### 🐛 Report a Bug
- Search [existing issues](https://github.com/uddi-protocol/uddi/issues) first
- Use the **Bug Report** template
- Include: OS, Node/Go/Rust version, steps to reproduce, expected vs actual behavior

### 💡 Suggest a Feature
- Open a [Feature Request](https://github.com/uddi-protocol/uddi/issues/new?template=feature_request.md)
- Describe the use case and why it matters for the protocol

### 🔐 Report a Security Vulnerability
**DO NOT open a public issue for security vulnerabilities.**
Email `security@uddi.network` with details. We respond within 48 hours.

### 📝 Improve Documentation
Docs currently live in the root `README.md` and package-level README files. PRs for typos, clarity, and examples are always welcome.

### 💻 Write Code
Check the [good first issue](https://github.com/uddi-protocol/uddi/labels/good%20first%20issue) label for beginner-friendly tasks.

---

## Development Setup

### Prerequisites

| Tool | Version | Used for |
|------|---------|----------|
| Node.js | ≥ 22.13 | TypeScript packages |
| pnpm | 11.1.1 | Package manager |
| Go | 1.25.x | API Gateway |
| Rust | stable | Planned blockchain node |
| Docker | latest | Planned local infrastructure |

### Setup

```bash
# 1. Fork the repo on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/uddi.git
cd uddi

# 2. Add upstream remote
git remote add upstream https://github.com/uddi-protocol/uddi.git

# 3. Enable pnpm and install TypeScript dependencies
corepack enable pnpm
pnpm install

# 4. Install Go dependencies
cd packages/api && go mod download && cd ../..

# 5. Run checks
pnpm -r build
pnpm -r test
pnpm -r lint
cd packages/api && go test ./...
cd packages/api && go vet ./...
```

---

## Project Structure

```
uddi/
├── packages/
│   ├── core/          # @uddi/core — TypeScript
│   │   └── src/
│   │       ├── types/      # All type definitions
│   │       ├── identity/   # DID generation & key management
│   │       ├── vc/         # Verifiable Credentials
│   │
│   ├── sdk/           # @uddi/sdk — TypeScript
│   │   └── src/
│   │       └── index.ts    # UddiClient and UddiVerifier
│   │
│   ├── api/           # API Gateway — Go
│   │   └── internal/
│   │       ├── server/     # Router wiring and HTTP tests
│   │       ├── handlers/   # HTTP handlers
│   │       ├── blockchain/ # Blockchain client
│   │       ├── config/     # Environment config
│   │       ├── response/   # JSON helpers
│   │       ├── middleware/ # HTTP middleware
│   │       └── zkp/        # ZKP service client
│   │
│   └── zkp/           # Zero-Knowledge Proofs
│       └── circuits/   # circom circuits
│
├── infra/             # Docker compose draft
└── scripts/           # Dev & deployment scripts
```

---

## Submitting a Pull Request

1. **Create a branch** from `develop` (not `main`):
   ```bash
   git checkout -b feat/my-feature develop
   ```

2. **Make your changes** following the coding standards below

3. **Write tests** for new functionality

4. **Run the full test suite**:
   ```bash
   pnpm test
   cd packages/api && go test ./...
   ```

5. **Commit** using [Conventional Commits](#commit-convention)

6. **Push** and open a PR against `develop`:
   ```bash
   git push origin feat/my-feature
   ```

7. Fill in the **PR template** with:
   - What changed and why
   - How to test it
   - Any breaking changes

### PR Review Process
- A maintainer will review within 5 business days
- Address all review comments
- Once approved, a maintainer will squash-merge

---

## Coding Standards

### TypeScript
- **Strict mode** enabled (`"strict": true`)
- Use `type` for type aliases, `interface` for object shapes
- Prefer `async/await` over raw Promises
- Document all public APIs with JSDoc
- No `any` — use `unknown` and narrow types

### Go
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `slog` for structured logging
- Return errors, don't panic in library code
- All exported functions have godoc comments

### Rust
- Run `cargo clippy` before committing (no warnings allowed)
- Use `thiserror` for error types
- Prefer `?` over `.unwrap()` in library code

### circom
- Comment every template with input/output descriptions
- Include security constraints (bounds checks)
- Document what the proof reveals and what it hides

---

## Commit Convention

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short description>

[optional body]

[optional footer]
```

**Types:**
- `feat` — new feature
- `fix` — bug fix
- `docs` — documentation only
- `test` — tests only
- `refactor` — code change without bug fix or feature
- `perf` — performance improvement
- `chore` — build process, tooling
- `security` — security fix (use for non-critical; critical → email us)

**Scopes:** `core`, `sdk`, `api`, `blockchain`, `zkp`, `mobile`, `docs`, `infra`

**Examples:**
```
feat(sdk): add UddiVerifier.verifyClaim method
fix(api): handle empty DID in resolution endpoint
docs(core): add JSDoc to generateIdentity function
feat(zkp): add citizenship verification circuit
```

---

## Questions?

- 💬 [GitHub Discussions](https://github.com/uddi-protocol/uddi/discussions)
- 📧 [hello@uddi.network](mailto:hello@uddi.network)
- 🔐 Security: [security@uddi.network](mailto:security@uddi.network)

**Thank you for helping build the future of digital identity!** 🌐
