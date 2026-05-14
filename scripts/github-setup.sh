#!/usr/bin/env bash
# =============================================================================
# UDDI Framework — GitHub Repository Setup Script
# Run this ONCE to initialize your GitHub repository
# Usage: chmod +x scripts/github-setup.sh && ./scripts/github-setup.sh
# =============================================================================

set -euo pipefail

# ── Colors ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log()    { echo -e "${GREEN}[✓]${NC} $1"; }
info()   { echo -e "${BLUE}[i]${NC} $1"; }
warn()   { echo -e "${YELLOW}[!]${NC} $1"; }
error()  { echo -e "${RED}[✗]${NC} $1"; exit 1; }

# ── Check Dependencies ────────────────────────────────────────────────────────
info "Checking dependencies..."
command -v git   >/dev/null 2>&1 || error "git is required"
command -v gh    >/dev/null 2>&1 || error "GitHub CLI (gh) is required. Install: https://cli.github.com"
command -v node  >/dev/null 2>&1 || error "Node.js 18+ is required"
command -v pnpm  >/dev/null 2>&1 || warn "pnpm not found. Install: npm install -g pnpm"
log "Dependencies OK"

# ── Collect Info ──────────────────────────────────────────────────────────────
echo ""
echo -e "${BLUE}╔═══════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   UDDI Framework — GitHub Setup       ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════╝${NC}"
echo ""

read -p "GitHub organization/username (e.g. uddi-protocol): " GITHUB_ORG
read -p "Repository name (e.g. uddi): " REPO_NAME
read -p "Your name (for git config): " GIT_NAME
read -p "Your email (for git config): " GIT_EMAIL

REPO_FULL="$GITHUB_ORG/$REPO_NAME"

# ── Git Configuration ─────────────────────────────────────────────────────────
info "Configuring git..."
git config user.name "$GIT_NAME"
git config user.email "$GIT_EMAIL"
log "Git configured"

# ── Initialize Repository ─────────────────────────────────────────────────────
info "Initializing git repository..."
git init
git add .
git commit -m "feat: initial UDDI framework scaffold

- @uddi/core: identity generation, DID, Verifiable Credentials
- @uddi/sdk: UddiClient + UddiVerifier
- @uddi/api: Go REST/gRPC API Gateway
- @uddi/blockchain: Substrate node scaffold
- @uddi/zkp: circom circuits (age, citizenship)
- infra: Docker Compose dev stack
- CI: GitHub Actions (TypeScript, Go, Rust)
- docs: README, CONTRIBUTING, whitepaper"

log "Initial commit created"

# ── Create GitHub Repository ──────────────────────────────────────────────────
info "Creating GitHub repository: $REPO_FULL..."
gh repo create "$REPO_FULL" \
  --public \
  --description "Universal Decentralized Digital Identity Protocol — Open Source" \
  --homepage "https://uddi.network" \
  --push \
  --source . \
  --remote origin

log "Repository created at https://github.com/$REPO_FULL"

# ── Setup Repository Settings ─────────────────────────────────────────────────
info "Configuring repository settings..."

# Set topics/tags
gh repo edit "$REPO_FULL" \
  --add-topic "decentralized-identity" \
  --add-topic "did" \
  --add-topic "self-sovereign-identity" \
  --add-topic "zero-knowledge-proof" \
  --add-topic "blockchain" \
  --add-topic "privacy" \
  --add-topic "open-source" \
  --add-topic "web3" \
  --add-topic "typescript" \
  --add-topic "rust" \
  --add-topic "go"

# Enable features
gh repo edit "$REPO_FULL" \
  --enable-issues \
  --enable-wiki \
  --enable-discussions

log "Repository settings configured"

# ── Create Branch Protection ──────────────────────────────────────────────────
info "Setting up branch protection..."

# Protect main branch
gh api repos/$REPO_FULL/branches/main/protection \
  --method PUT \
  --field required_status_checks='{"strict":true,"contexts":["typescript","golang","rust"]}' \
  --field enforce_admins=false \
  --field required_pull_request_reviews='{"required_approving_review_count":1}' \
  --field restrictions=null \
  2>/dev/null && log "Branch protection enabled" || warn "Branch protection skipped (may need admin rights)"

# ── Create develop branch ─────────────────────────────────────────────────────
info "Creating develop branch..."
git checkout -b develop
git push origin develop
git checkout main
log "develop branch created"

# ── Create GitHub Labels ──────────────────────────────────────────────────────
info "Creating issue labels..."

create_label() {
  gh label create "$1" --color "$2" --description "$3" --repo "$REPO_FULL" 2>/dev/null || true
}

create_label "good first issue"     "7057ff" "Good for newcomers"
create_label "help wanted"          "008672" "Extra attention needed"
create_label "bug"                  "d73a4a" "Something isn't working"
create_label "enhancement"          "a2eeef" "New feature or request"
create_label "security"             "e4e669" "Security related"
create_label "documentation"        "0075ca" "Improvements to docs"
create_label "core"                 "1d76db" "Core cryptography & DID"
create_label "sdk"                  "0e8a16" "TypeScript SDK"
create_label "api"                  "fbca04" "Go API Gateway"
create_label "blockchain"           "b60205" "Substrate blockchain"
create_label "zkp"                  "5319e7" "Zero-Knowledge Proofs"
create_label "mobile"               "e99695" "React Native app"
create_label "breaking change"      "b60205" "Breaking API change"
create_label "needs review"         "ededed" "Waiting for review"

log "Labels created"

# ── Create Milestones ─────────────────────────────────────────────────────────
info "Creating milestones..."

gh api repos/$REPO_FULL/milestones \
  --method POST \
  --field title="v0.1.0 - Foundation" \
  --field description="Core protocol, SDK, and API Gateway" \
  2>/dev/null || true

gh api repos/$REPO_FULL/milestones \
  --method POST \
  --field title="v0.2.0 - ZKP & Blockchain" \
  --field description="ZKP circuits, Substrate node, testnet" \
  2>/dev/null || true

gh api repos/$REPO_FULL/milestones \
  --method POST \
  --field title="v1.0.0 - Testnet Launch" \
  --field description="Public testnet, security audit, full documentation" \
  2>/dev/null || true

log "Milestones created"

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}╔═══════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║           UDDI Framework is live on GitHub!           ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  📦 Repository:  ${BLUE}https://github.com/$REPO_FULL${NC}"
echo -e "  📋 Issues:      ${BLUE}https://github.com/$REPO_FULL/issues${NC}"
echo -e "  💬 Discussions: ${BLUE}https://github.com/$REPO_FULL/discussions${NC}"
echo ""
echo -e "  ${YELLOW}Next steps:${NC}"
echo -e "  1. Star the repo ⭐ and share it!"
echo -e "  2. Run: pnpm install && pnpm build"
echo -e "  3. Start local infra: docker-compose -f infra/docker/docker-compose.dev.yml up -d"
echo -e "  4. Start building 🚀"
echo ""
