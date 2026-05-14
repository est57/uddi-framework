package server_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/uddi-protocol/uddi/api/internal/blockchain"
	"github.com/uddi-protocol/uddi/api/internal/config"
	"github.com/uddi-protocol/uddi/api/internal/server"
	"github.com/uddi-protocol/uddi/api/internal/zkp"
)

func TestHealth(t *testing.T) {
	router := newTestRouter(t)

	res := performRequest(router, http.MethodGet, "/health", nil, nil)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	assertJSONField(t, res.Body.Bytes(), "status", "ok")
	if res.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected security headers to be set")
	}
}

func TestReadinessAndMetrics(t *testing.T) {
	router := newTestRouter(t)

	readyRes := performRequest(router, http.MethodGet, "/ready", nil, nil)
	if readyRes.Code != http.StatusOK {
		t.Fatalf("expected ready status 200, got %d", readyRes.Code)
	}
	assertJSONField(t, readyRes.Body.Bytes(), "status", "ready")

	metricsRes := performRequest(router, http.MethodGet, "/metrics", nil, nil)
	if metricsRes.Code != http.StatusOK {
		t.Fatalf("expected metrics status 200, got %d", metricsRes.Code)
	}
	assertJSONField(t, metricsRes.Body.Bytes(), "metricsContentType", "application/json")
	assertJSONNumberAtLeast(t, metricsRes.Body.Bytes(), "requestsTotal", 2)
}

func TestRegistryStatsUsesDIDStoreState(t *testing.T) {
	router := newTestRouter(t)
	identity := newTestIdentityWithSuffix(t, "stats123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")
	registerDID(t, router, identity)

	statsRes := performRequest(router, http.MethodGet, "/v1/registry/stats", nil, nil)
	if statsRes.Code != http.StatusOK {
		t.Fatalf("expected stats status 200, got %d: %s", statsRes.Code, statsRes.Body.String())
	}
	assertJSONNumberAtLeast(t, statsRes.Body.Bytes(), "totalDIDs", 1)
	assertJSONNumberAtLeast(t, statsRes.Body.Bytes(), "activeDIDs", 1)
	assertJSONField(t, statsRes.Body.Bytes(), "backend", "memory")
}

func TestAPIKeyMiddleware(t *testing.T) {
	router := newTestRouter(t)

	res := performRequest(router, http.MethodGet, "/v1/credentials/did:uddi:z123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij", nil, nil)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", res.Code)
	}
	assertJSONField(t, res.Body.Bytes(), "error", "missing API key")
}

func TestAdminAPIKeyLifecycle(t *testing.T) {
	router := newTestRouterWithConfig(t, &config.Config{
		AdminToken:     "admin-token",
		AllowedOrigins: []string{"*"},
	})

	unauthorizedRes := performRequest(router, http.MethodPost, "/v1/admin/api-keys/", map[string]any{
		"serviceId": "production-service",
	}, nil)
	if unauthorizedRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status 401, got %d: %s", unauthorizedRes.Code, unauthorizedRes.Body.String())
	}

	createRes := performRequest(router, http.MethodPost, "/v1/admin/api-keys/", map[string]any{
		"serviceId":   "production-service",
		"serviceName": "Production Service",
	}, adminHeaders())
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected create status 201, got %d: %s", createRes.Code, createRes.Body.String())
	}

	var createPayload struct {
		APIKey string `json:"apiKey"`
		Record struct {
			ServiceID string `json:"serviceId"`
		} `json:"record"`
	}
	if err := json.Unmarshal(createRes.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createPayload.APIKey == "" {
		t.Fatalf("expected generated API key")
	}
	if createPayload.Record.ServiceID != "production-service" {
		t.Fatalf("expected production-service record, got %s", createPayload.Record.ServiceID)
	}

	challengeRes := performRequest(router, http.MethodPost, "/v1/verify/challenge", map[string]any{
		"serviceId": "production-service",
	}, map[string]string{
		"X-Service-ID": "production-service",
		"X-API-Key":    createPayload.APIKey,
	})
	if challengeRes.Code != http.StatusCreated {
		t.Fatalf("expected new key to authorize challenge, got %d: %s", challengeRes.Code, challengeRes.Body.String())
	}

	listRes := performRequest(router, http.MethodGet, "/v1/admin/api-keys/", nil, adminHeaders())
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d: %s", listRes.Code, listRes.Body.String())
	}

	revokeRes := performRequest(router, http.MethodPost, "/v1/admin/api-keys/revoke", map[string]any{
		"serviceId": "production-service",
	}, adminHeaders())
	if revokeRes.Code != http.StatusOK {
		t.Fatalf("expected revoke status 200, got %d: %s", revokeRes.Code, revokeRes.Body.String())
	}

	retryRes := performRequest(router, http.MethodPost, "/v1/verify/challenge", map[string]any{
		"serviceId": "production-service",
	}, map[string]string{
		"X-Service-ID": "production-service",
		"X-API-Key":    createPayload.APIKey,
	})
	if retryRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked key status 401, got %d: %s", retryRes.Code, retryRes.Body.String())
	}
}

func TestRateLimit(t *testing.T) {
	router := newTestRouterWithConfig(t, &config.Config{
		AllowedOrigins:      []string{"*"},
		RateLimitRequests:   1,
		RateLimitWindow:     time.Minute,
		MaxRequestBodyBytes: 1_048_576,
	})

	firstRes := performRequest(router, http.MethodGet, "/health", nil, nil)
	if firstRes.Code != http.StatusOK {
		t.Fatalf("expected first request status 200, got %d", firstRes.Code)
	}

	secondRes := performRequest(router, http.MethodGet, "/health", nil, nil)
	if secondRes.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request status 429, got %d: %s", secondRes.Code, secondRes.Body.String())
	}
	assertJSONField(t, secondRes.Body.Bytes(), "error", "rate limit exceeded")
}

func TestDIDLifecycle(t *testing.T) {
	router := newTestRouter(t)
	identity := newTestIdentity(t)
	updatedIdentity := newTestIdentityWithSuffix(t, "updated123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")

	registerRes := registerDID(t, router, identity)
	if registerRes.Code != http.StatusCreated {
		t.Fatalf("expected register status 201, got %d: %s", registerRes.Code, registerRes.Body.String())
	}

	resolveRes := performRequest(router, http.MethodGet, "/v1/did/"+identity.did, nil, nil)
	if resolveRes.Code != http.StatusOK {
		t.Fatalf("expected resolve status 200, got %d: %s", resolveRes.Code, resolveRes.Body.String())
	}
	assertNestedJSONField(t, resolveRes.Body.Bytes(), []string{"didDocument", "id"}, identity.did)

	updateTimestamp := "1700000000002"
	updatedPublicKey := base64.StdEncoding.EncodeToString(updatedIdentity.publicKey)
	updateRes := performRequest(router, http.MethodPut, "/v1/did/"+identity.did+"/update", map[string]any{
		"did":             identity.did,
		"publicKeyBase64": updatedPublicKey,
		"context":         []string{"https://www.w3.org/ns/did/v1", "https://uddi.network/v1"},
		"signatureBase64": signBase64(t, identity.privateKey, "update:"+identity.did+":"+updatedPublicKey+":"+updateTimestamp),
		"timestamp":       updateTimestamp,
	}, nil)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("expected update status 200, got %d: %s", updateRes.Code, updateRes.Body.String())
	}
	assertJSONField(t, updateRes.Body.Bytes(), "status", "UPDATED")

	resolveUpdatedRes := performRequest(router, http.MethodGet, "/v1/did/"+identity.did, nil, nil)
	assertNestedJSONField(t, resolveUpdatedRes.Body.Bytes(), []string{"didDocument", "publicKeyBase64"}, updatedPublicKey)

	revokeTimestamp := "1700000000001"
	revokeBody := map[string]any{
		"did":             identity.did,
		"signatureBase64": signBase64(t, updatedIdentity.privateKey, "revoke:"+identity.did+":"+revokeTimestamp),
		"timestamp":       revokeTimestamp,
	}
	revokeRes := performRequest(router, http.MethodPost, "/v1/did/revoke", revokeBody, nil)
	if revokeRes.Code != http.StatusOK {
		t.Fatalf("expected revoke status 200, got %d: %s", revokeRes.Code, revokeRes.Body.String())
	}

	resolveRevokedRes := performRequest(router, http.MethodGet, "/v1/did/"+identity.did, nil, nil)
	assertNestedJSONField(t, resolveRevokedRes.Body.Bytes(), []string{"didDocument", "deactivated"}, true)
}

func TestDIDUpdateRejectsInvalidRequests(t *testing.T) {
	router := newTestRouter(t)
	identity := newTestIdentity(t)
	newIdentity := newTestIdentityWithSuffix(t, "newkey123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")
	registerDID(t, router, identity)

	timestamp := "1700000000003"
	newPublicKey := base64.StdEncoding.EncodeToString(newIdentity.publicKey)

	mismatchRes := performRequest(router, http.MethodPut, "/v1/did/"+identity.did+"/update", map[string]any{
		"did":             newIdentity.did,
		"publicKeyBase64": newPublicKey,
		"signatureBase64": signBase64(t, identity.privateKey, "update:"+identity.did+":"+newPublicKey+":"+timestamp),
		"timestamp":       timestamp,
	}, nil)
	if mismatchRes.Code != http.StatusBadRequest {
		t.Fatalf("expected mismatch status 400, got %d: %s", mismatchRes.Code, mismatchRes.Body.String())
	}

	wrongSignatureRes := performRequest(router, http.MethodPut, "/v1/did/"+identity.did+"/update", map[string]any{
		"did":             identity.did,
		"publicKeyBase64": newPublicKey,
		"signatureBase64": signBase64(t, newIdentity.privateKey, "update:"+identity.did+":"+newPublicKey+":"+timestamp),
		"timestamp":       timestamp,
	}, nil)
	if wrongSignatureRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected wrong signature status 401, got %d: %s", wrongSignatureRes.Code, wrongSignatureRes.Body.String())
	}
	assertJSONField(t, wrongSignatureRes.Body.Bytes(), "error", "signature verification failed")

	revokeTimestamp := "1700000000004"
	revokeRes := performRequest(router, http.MethodPost, "/v1/did/revoke", map[string]any{
		"did":             identity.did,
		"signatureBase64": signBase64(t, identity.privateKey, "revoke:"+identity.did+":"+revokeTimestamp),
		"timestamp":       revokeTimestamp,
	}, nil)
	if revokeRes.Code != http.StatusOK {
		t.Fatalf("expected revoke status 200, got %d: %s", revokeRes.Code, revokeRes.Body.String())
	}

	deactivatedRes := performRequest(router, http.MethodPut, "/v1/did/"+identity.did+"/update", map[string]any{
		"did":             identity.did,
		"publicKeyBase64": newPublicKey,
		"signatureBase64": signBase64(t, identity.privateKey, "update:"+identity.did+":"+newPublicKey+":"+timestamp),
		"timestamp":       timestamp,
	}, nil)
	if deactivatedRes.Code != http.StatusConflict {
		t.Fatalf("expected deactivated status 409, got %d: %s", deactivatedRes.Code, deactivatedRes.Body.String())
	}
	assertJSONField(t, deactivatedRes.Body.Bytes(), "error", "DID is deactivated")
}

func TestAuthChallengeAndVerification(t *testing.T) {
	router := newTestRouter(t)
	identity := newTestIdentity(t)
	registerDID(t, router, identity)

	challengeRes := performRequest(router, http.MethodPost, "/v1/verify/challenge", map[string]any{
		"serviceId":   "test-service",
		"serviceName": "Test Service",
	}, apiHeaders())
	if challengeRes.Code != http.StatusCreated {
		t.Fatalf("expected challenge status 201, got %d: %s", challengeRes.Code, challengeRes.Body.String())
	}

	var challenge struct {
		ChallengeID string `json:"challengeId"`
		Nonce       string `json:"nonce"`
	}
	if err := json.Unmarshal(challengeRes.Body.Bytes(), &challenge); err != nil {
		t.Fatalf("decode challenge: %v", err)
	}

	timestamp := time.Now().UnixMilli()
	message := challenge.ChallengeID + ":" + challenge.Nonce + ":" + identity.did + ":" + int64String(timestamp)
	presentationPayload := map[string]any{
		"did":         identity.did,
		"challengeId": challenge.ChallengeID,
		"signature":   signBase64(t, identity.privateKey, message),
		"timestamp":   timestamp,
	}
	presentationBytes, err := json.Marshal(presentationPayload)
	if err != nil {
		t.Fatalf("marshal presentation: %v", err)
	}

	verifyRes := performRequest(router, http.MethodPost, "/v1/verify/auth", map[string]any{
		"challengeId":  challenge.ChallengeID,
		"presentation": base64.StdEncoding.EncodeToString(presentationBytes),
	}, apiHeaders())
	if verifyRes.Code != http.StatusOK {
		t.Fatalf("expected verify status 200, got %d: %s", verifyRes.Code, verifyRes.Body.String())
	}
	assertJSONField(t, verifyRes.Body.Bytes(), "valid", true)
	assertJSONField(t, verifyRes.Body.Bytes(), "did", identity.did)

	replayRes := performRequest(router, http.MethodPost, "/v1/verify/auth", map[string]any{
		"challengeId":  challenge.ChallengeID,
		"presentation": base64.StdEncoding.EncodeToString(presentationBytes),
	}, apiHeaders())
	assertJSONField(t, replayRes.Body.Bytes(), "valid", false)
	assertJSONField(t, replayRes.Body.Bytes(), "reason", "challenge not found or presentation missing")
}

func TestAuthRejectsMismatchedPresentation(t *testing.T) {
	router := newTestRouter(t)
	identity := newTestIdentity(t)
	registerDID(t, router, identity)

	challengeRes := performRequest(router, http.MethodPost, "/v1/verify/challenge", map[string]any{
		"serviceId": "test-service",
	}, apiHeaders())

	var challenge struct {
		ChallengeID string `json:"challengeId"`
		Nonce       string `json:"nonce"`
	}
	if err := json.Unmarshal(challengeRes.Body.Bytes(), &challenge); err != nil {
		t.Fatalf("decode challenge: %v", err)
	}

	timestamp := time.Now().UnixMilli()
	message := challenge.ChallengeID + ":" + challenge.Nonce + ":" + identity.did + ":" + int64String(timestamp)
	presentationPayload := map[string]any{
		"did":         identity.did,
		"challengeId": "different-challenge",
		"signature":   signBase64(t, identity.privateKey, message),
		"timestamp":   timestamp,
	}
	presentationBytes, err := json.Marshal(presentationPayload)
	if err != nil {
		t.Fatalf("marshal presentation: %v", err)
	}

	verifyRes := performRequest(router, http.MethodPost, "/v1/verify/auth", map[string]any{
		"challengeId":  challenge.ChallengeID,
		"presentation": base64.StdEncoding.EncodeToString(presentationBytes),
	}, apiHeaders())

	assertJSONField(t, verifyRes.Body.Bytes(), "valid", false)
	assertJSONField(t, verifyRes.Body.Bytes(), "reason", "challenge mismatch")
}

func TestAuthRejectsServiceMismatchAndConsumesChallenge(t *testing.T) {
	router := newTestRouter(t)
	identity := newTestIdentity(t)
	registerDID(t, router, identity)
	challenge := createAuthChallenge(t, router)
	presentation := signedPresentation(t, identity, challenge, time.Now().UnixMilli(), challenge.ChallengeID)

	verifyRes := performRequest(router, http.MethodPost, "/v1/verify/auth", map[string]any{
		"challengeId":  challenge.ChallengeID,
		"presentation": presentation,
		"serviceId":    "wrong-service",
	}, apiHeaders())
	assertJSONField(t, verifyRes.Body.Bytes(), "valid", false)
	assertJSONField(t, verifyRes.Body.Bytes(), "reason", "service mismatch")

	retryRes := performRequest(router, http.MethodPost, "/v1/verify/auth", map[string]any{
		"challengeId":  challenge.ChallengeID,
		"presentation": presentation,
		"serviceId":    "test-service",
	}, apiHeaders())
	assertJSONField(t, retryRes.Body.Bytes(), "valid", false)
	assertJSONField(t, retryRes.Body.Bytes(), "reason", "challenge not found or presentation missing")
}

func TestAuthRejectsStalePresentationTimestamp(t *testing.T) {
	router := newTestRouter(t)
	identity := newTestIdentity(t)
	registerDID(t, router, identity)
	challenge := createAuthChallenge(t, router)
	oldTimestamp := time.Now().Add(-6 * time.Minute).UnixMilli()

	verifyRes := performRequest(router, http.MethodPost, "/v1/verify/auth", map[string]any{
		"challengeId":  challenge.ChallengeID,
		"presentation": signedPresentation(t, identity, challenge, oldTimestamp, challenge.ChallengeID),
		"serviceId":    "test-service",
	}, apiHeaders())

	assertJSONField(t, verifyRes.Body.Bytes(), "valid", false)
	assertJSONField(t, verifyRes.Body.Bytes(), "reason", "presentation timestamp outside allowed window")
}

func TestAuthRejectsFuturePresentationTimestamp(t *testing.T) {
	router := newTestRouter(t)
	identity := newTestIdentity(t)
	registerDID(t, router, identity)
	challenge := createAuthChallenge(t, router)
	futureTimestamp := time.Now().Add(45 * time.Second).UnixMilli()

	verifyRes := performRequest(router, http.MethodPost, "/v1/verify/auth", map[string]any{
		"challengeId":  challenge.ChallengeID,
		"presentation": signedPresentation(t, identity, challenge, futureTimestamp, challenge.ChallengeID),
		"serviceId":    "test-service",
	}, apiHeaders())

	assertJSONField(t, verifyRes.Body.Bytes(), "valid", false)
	assertJSONField(t, verifyRes.Body.Bytes(), "reason", "presentation timestamp outside allowed window")
}

func TestAuthRejectsInvalidSignatureAndConsumesChallenge(t *testing.T) {
	router := newTestRouter(t)
	identity := newTestIdentity(t)
	otherIdentity := newTestIdentity(t)
	registerDID(t, router, identity)
	challenge := createAuthChallenge(t, router)
	timestamp := time.Now().UnixMilli()
	message := challenge.ChallengeID + ":" + challenge.Nonce + ":" + identity.did + ":" + int64String(timestamp)
	presentationPayload := map[string]any{
		"did":         identity.did,
		"challengeId": challenge.ChallengeID,
		"signature":   signBase64(t, otherIdentity.privateKey, message),
		"timestamp":   timestamp,
	}
	presentationBytes, err := json.Marshal(presentationPayload)
	if err != nil {
		t.Fatalf("marshal presentation: %v", err)
	}
	presentation := base64.StdEncoding.EncodeToString(presentationBytes)

	verifyRes := performRequest(router, http.MethodPost, "/v1/verify/auth", map[string]any{
		"challengeId":  challenge.ChallengeID,
		"presentation": presentation,
		"serviceId":    "test-service",
	}, apiHeaders())
	assertJSONField(t, verifyRes.Body.Bytes(), "valid", false)
	assertJSONField(t, verifyRes.Body.Bytes(), "reason", "invalid signature")

	retryRes := performRequest(router, http.MethodPost, "/v1/verify/auth", map[string]any{
		"challengeId":  challenge.ChallengeID,
		"presentation": signedPresentation(t, identity, challenge, time.Now().UnixMilli(), challenge.ChallengeID),
		"serviceId":    "test-service",
	}, apiHeaders())
	assertJSONField(t, retryRes.Body.Bytes(), "valid", false)
	assertJSONField(t, retryRes.Body.Bytes(), "reason", "challenge not found or presentation missing")
}

func TestProofGeneration(t *testing.T) {
	router := newTestRouter(t)

	res := performRequest(router, http.MethodPost, "/v1/proof/generate", map[string]any{
		"did":    "did:uddi:z123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij",
		"type":   "age",
		"params": map[string]any{"minimumAge": 18},
	}, nil)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	assertNestedJSONField(t, res.Body.Bytes(), []string{"proof", "type"}, "age")
	assertNestedJSONField(t, res.Body.Bytes(), []string{"proof", "circuit"}, "age_verification")
}

func TestProofGenerationRequiresType(t *testing.T) {
	router := newTestRouter(t)

	res := performRequest(router, http.MethodPost, "/v1/proof/generate", map[string]any{
		"did":    "did:uddi:z123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij",
		"params": map[string]any{"minimumAge": 18},
	}, nil)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", res.Code, res.Body.String())
	}
	assertJSONField(t, res.Body.Bytes(), "error", "proof type is required")
}

func TestClaimVerificationValidatesProofShape(t *testing.T) {
	router := newTestRouter(t)

	proofRes := performRequest(router, http.MethodPost, "/v1/proof/generate", map[string]any{
		"did":    "did:uddi:z123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij",
		"type":   "age",
		"params": map[string]any{"minimumAge": 18},
	}, nil)
	var proofPayload struct {
		Proof map[string]any `json:"proof"`
	}
	if err := json.Unmarshal(proofRes.Body.Bytes(), &proofPayload); err != nil {
		t.Fatalf("decode proof response: %v", err)
	}

	validRes := performRequest(router, http.MethodPost, "/v1/verify/claim", map[string]any{
		"did":       "did:uddi:z123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij",
		"claimType": "age",
		"proof":     proofPayload.Proof,
	}, apiHeaders())
	if validRes.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", validRes.Code, validRes.Body.String())
	}
	assertJSONField(t, validRes.Body.Bytes(), "valid", true)

	invalidRes := performRequest(router, http.MethodPost, "/v1/verify/claim", map[string]any{
		"did":       "did:uddi:z123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij",
		"claimType": "age",
		"proof": map[string]any{
			"protocol": "groth16",
			"curve":    "bn128",
			"type":     "citizenship",
			"proof":    map[string]any{},
		},
	}, apiHeaders())
	assertJSONField(t, invalidRes.Body.Bytes(), "valid", false)
	assertJSONField(t, invalidRes.Body.Bytes(), "reason", "proof type does not match claim type")
}

func TestCredentialLifecycle(t *testing.T) {
	router := newTestRouter(t)
	issuer := newTestIdentityWithSuffix(t, "issuer123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")
	subject := newTestIdentityWithSuffix(t, "subject12345678ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")
	registerDID(t, router, issuer)
	registerDID(t, router, subject)

	credential := signedCredential(t, issuer, subject, "urn:uddi:vc:test-credential")

	issueRes := performRequest(router, http.MethodPost, "/v1/credentials/issue", map[string]any{
		"credential": credential,
	}, apiHeaders())
	if issueRes.Code != http.StatusCreated {
		t.Fatalf("expected issue status 201, got %d: %s", issueRes.Code, issueRes.Body.String())
	}
	assertJSONField(t, issueRes.Body.Bytes(), "status", "ISSUED")

	listRes := performRequest(router, http.MethodGet, "/v1/credentials/"+subject.did, nil, apiHeaders())
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d: %s", listRes.Code, listRes.Body.String())
	}
	assertNestedJSONField(t, listRes.Body.Bytes(), []string{"credentials", "0", "id"}, "urn:uddi:vc:test-credential")

	verifyRes := performRequest(router, http.MethodGet, "/v1/credentials/urn:uddi:vc:test-credential/verify", nil, apiHeaders())
	assertJSONField(t, verifyRes.Body.Bytes(), "valid", true)

	revokeRes := performRequest(router, http.MethodPost, "/v1/credentials/revoke", map[string]any{
		"id":     "urn:uddi:vc:test-credential",
		"reason": "test revocation",
	}, apiHeaders())
	if revokeRes.Code != http.StatusOK {
		t.Fatalf("expected revoke status 200, got %d: %s", revokeRes.Code, revokeRes.Body.String())
	}
	assertJSONField(t, revokeRes.Body.Bytes(), "status", "REVOKED")

	verifyRevokedRes := performRequest(router, http.MethodGet, "/v1/credentials/urn:uddi:vc:test-credential/verify", nil, apiHeaders())
	assertJSONField(t, verifyRevokedRes.Body.Bytes(), "valid", false)
	assertJSONField(t, verifyRevokedRes.Body.Bytes(), "reason", "credential revoked")
}

func TestCredentialIssueRejectsInvalidProof(t *testing.T) {
	router := newTestRouter(t)
	issuer := newTestIdentityWithSuffix(t, "issuerinvalidABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")
	subject := newTestIdentityWithSuffix(t, "subjectinvalidABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")
	registerDID(t, router, issuer)
	registerDID(t, router, subject)

	credential := signedCredential(t, issuer, subject, "urn:uddi:vc:invalid-proof")
	credential["proof"].(map[string]any)["proofValue"] = signBase64(t, issuer.privateKey, "wrong-message")

	issueRes := performRequest(router, http.MethodPost, "/v1/credentials/issue", map[string]any{
		"credential": credential,
	}, apiHeaders())
	if issueRes.Code != http.StatusBadRequest {
		t.Fatalf("expected issue status 400, got %d: %s", issueRes.Code, issueRes.Body.String())
	}
	assertJSONField(t, issueRes.Body.Bytes(), "error", "invalid credential proof")
}

func TestCredentialVerifyReportsExpiredCredential(t *testing.T) {
	router := newTestRouter(t)
	issuer := newTestIdentityWithSuffix(t, "issuerexpiredABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")
	subject := newTestIdentityWithSuffix(t, "subjectexpiredABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")
	registerDID(t, router, issuer)
	registerDID(t, router, subject)

	credential := signedCredentialWithDates(
		t,
		issuer,
		subject,
		"urn:uddi:vc:expired",
		time.Now().UTC().Add(-48*time.Hour),
		time.Now().UTC().Add(-24*time.Hour),
	)

	issueRes := performRequest(router, http.MethodPost, "/v1/credentials/issue", map[string]any{
		"credential": credential,
	}, apiHeaders())
	if issueRes.Code != http.StatusCreated {
		t.Fatalf("expected issue status 201, got %d: %s", issueRes.Code, issueRes.Body.String())
	}

	verifyRes := performRequest(router, http.MethodGet, "/v1/credentials/urn:uddi:vc:expired/verify", nil, apiHeaders())
	assertJSONField(t, verifyRes.Body.Bytes(), "valid", false)
	assertJSONField(t, verifyRes.Body.Bytes(), "reason", "credential expired")
}

func TestCredentialIssueValidatesDates(t *testing.T) {
	router := newTestRouter(t)
	issuer := newTestIdentityWithSuffix(t, "issuerdateABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")
	subject := newTestIdentityWithSuffix(t, "subjectdateABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")
	registerDID(t, router, issuer)
	registerDID(t, router, subject)

	credential := signedCredential(t, issuer, subject, "urn:uddi:vc:invalid-date")
	credential["expirationDate"] = "not-a-date"

	issueRes := performRequest(router, http.MethodPost, "/v1/credentials/issue", map[string]any{
		"credential": credential,
	}, apiHeaders())
	if issueRes.Code != http.StatusBadRequest {
		t.Fatalf("expected issue status 400, got %d: %s", issueRes.Code, issueRes.Body.String())
	}
	assertJSONField(t, issueRes.Body.Bytes(), "error", "credential.expirationDate must be RFC3339")

	reversedDatesCredential := signedCredentialWithDates(
		t,
		issuer,
		subject,
		"urn:uddi:vc:reversed-dates",
		time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 14, 9, 0, 0, 0, time.UTC),
	)
	reversedDatesRes := performRequest(router, http.MethodPost, "/v1/credentials/issue", map[string]any{
		"credential": reversedDatesCredential,
	}, apiHeaders())
	if reversedDatesRes.Code != http.StatusBadRequest {
		t.Fatalf("expected issue status 400, got %d: %s", reversedDatesRes.Code, reversedDatesRes.Body.String())
	}
	assertJSONField(t, reversedDatesRes.Body.Bytes(), "error", "credential.expirationDate must be after issuanceDate")
}

type testIdentity struct {
	did        string
	publicKey  ed25519.PublicKey
	privateKey ed25519.PrivateKey
}

type testChallenge struct {
	ChallengeID string `json:"challengeId"`
	Nonce       string `json:"nonce"`
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()

	return newTestRouterWithConfig(t, &config.Config{
		AllowedOrigins: []string{"*"},
	})
}

func newTestRouterWithConfig(t *testing.T, cfg *config.Config) http.Handler {
	t.Helper()

	if len(cfg.AllowedOrigins) == 0 {
		cfg.AllowedOrigins = []string{"*"}
	}
	if cfg.MaxRequestBodyBytes == 0 {
		cfg.MaxRequestBodyBytes = 1_048_576
	}
	if cfg.RateLimitRequests == 0 {
		cfg.RateLimitRequests = 120
	}
	if cfg.RateLimitWindow == 0 {
		cfg.RateLimitWindow = time.Minute
	}
	chainClient, err := blockchain.NewClient("memory://test")
	if err != nil {
		t.Fatalf("new blockchain client: %v", err)
	}
	router, err := server.NewRouter(cfg, chainClient, zkp.NewService("memory://zkp"))
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	return router
}

func newTestIdentity(t *testing.T) testIdentity {
	t.Helper()
	return newTestIdentityWithSuffix(t, "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghij")
}

func newTestIdentityWithSuffix(t *testing.T, suffix string) testIdentity {
	t.Helper()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return testIdentity{
		did:        "did:uddi:z" + suffix,
		publicKey:  publicKey,
		privateKey: privateKey,
	}
}

func registerDID(t *testing.T, router http.Handler, identity testIdentity) *httptest.ResponseRecorder {
	t.Helper()

	timestamp := "1700000000000"
	body := map[string]any{
		"did":             identity.did,
		"publicKeyBase64": base64.StdEncoding.EncodeToString(identity.publicKey),
		"signatureBase64": signBase64(t, identity.privateKey, "register:"+identity.did+":"+timestamp),
		"timestamp":       timestamp,
	}
	return performRequest(router, http.MethodPost, "/v1/did/register", body, nil)
}

func createAuthChallenge(t *testing.T, router http.Handler) testChallenge {
	t.Helper()

	challengeRes := performRequest(router, http.MethodPost, "/v1/verify/challenge", map[string]any{
		"serviceId":   "test-service",
		"serviceName": "Test Service",
	}, apiHeaders())
	if challengeRes.Code != http.StatusCreated {
		t.Fatalf("expected challenge status 201, got %d: %s", challengeRes.Code, challengeRes.Body.String())
	}

	var challenge testChallenge
	if err := json.Unmarshal(challengeRes.Body.Bytes(), &challenge); err != nil {
		t.Fatalf("decode challenge: %v", err)
	}
	return challenge
}

func signedPresentation(
	t *testing.T,
	identity testIdentity,
	challenge testChallenge,
	timestamp int64,
	presentationChallengeID string,
) string {
	t.Helper()

	message := challenge.ChallengeID + ":" + challenge.Nonce + ":" + identity.did + ":" + int64String(timestamp)
	presentationPayload := map[string]any{
		"did":         identity.did,
		"challengeId": presentationChallengeID,
		"signature":   signBase64(t, identity.privateKey, message),
		"timestamp":   timestamp,
	}
	presentationBytes, err := json.Marshal(presentationPayload)
	if err != nil {
		t.Fatalf("marshal presentation: %v", err)
	}
	return base64.StdEncoding.EncodeToString(presentationBytes)
}

func performRequest(router http.Handler, method string, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var requestBody bytes.Reader
	if body != nil {
		payload, _ := json.Marshal(body)
		requestBody = *bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, path, &requestBody)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func apiHeaders() map[string]string {
	return map[string]string{
		"X-API-Key":    "test-key",
		"X-Service-ID": "test-service",
	}
}

func adminHeaders() map[string]string {
	return map[string]string{
		"X-Admin-Token": "admin-token",
	}
}

func signBase64(t *testing.T, privateKey ed25519.PrivateKey, message string) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(message)))
}

func signedCredential(t *testing.T, issuer testIdentity, subject testIdentity, id string) map[string]any {
	t.Helper()

	return signedCredentialWithDates(t, issuer, subject, id, time.Now().UTC(), time.Time{})
}

func signedCredentialWithDates(
	t *testing.T,
	issuer testIdentity,
	subject testIdentity,
	id string,
	issuanceDate time.Time,
	expirationDate time.Time,
) map[string]any {
	t.Helper()

	credential := map[string]any{
		"@context":     []any{"https://www.w3.org/2018/credentials/v1"},
		"id":           id,
		"type":         []any{"VerifiableCredential", "AgeCredential"},
		"issuer":       issuer.did,
		"issuanceDate": issuanceDate.Format(time.RFC3339),
		"credentialSubject": map[string]any{
			"id":        subject.did,
			"birthYear": 2000,
		},
	}
	if !expirationDate.IsZero() {
		credential["expirationDate"] = expirationDate.Format(time.RFC3339)
	}
	message, err := canonicalizeForTest(credential)
	if err != nil {
		t.Fatalf("canonicalize credential: %v", err)
	}
	credential["proof"] = map[string]any{
		"type":               "Ed25519Signature2020",
		"verificationMethod": issuer.did + "#keys-1",
		"proofPurpose":       "assertionMethod",
		"proofValue":         signBase64(t, issuer.privateKey, message),
	}
	return credential
}

func canonicalizeForTest(value any) (string, error) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			keyJSON, err := json.Marshal(key)
			if err != nil {
				return "", err
			}
			valueJSON, err := canonicalizeForTest(typed[key])
			if err != nil {
				return "", err
			}
			buf.Write(keyJSON)
			buf.WriteByte(':')
			buf.WriteString(valueJSON)
		}
		buf.WriteByte('}')
		return buf.String(), nil
	case []any:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, item := range typed {
			if i > 0 {
				buf.WriteByte(',')
			}
			itemJSON, err := canonicalizeForTest(item)
			if err != nil {
				return "", err
			}
			buf.WriteString(itemJSON)
		}
		buf.WriteByte(']')
		return buf.String(), nil
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	}
}

func assertJSONField(t *testing.T, payload []byte, key string, expected any) {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if decoded[key] != expected {
		t.Fatalf("expected %s=%v, got %v in %s", key, expected, decoded[key], string(payload))
	}
}

func assertJSONNumberAtLeast(t *testing.T, payload []byte, key string, minimum float64) {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	value, ok := decoded[key].(float64)
	if !ok {
		t.Fatalf("expected numeric %s in %s", key, string(payload))
	}
	if value < minimum {
		t.Fatalf("expected %s >= %v, got %v in %s", key, minimum, value, string(payload))
	}
}

func assertNestedJSONField(t *testing.T, payload []byte, path []string, expected any) {
	t.Helper()

	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	for _, key := range path {
		if index, err := strconv.Atoi(key); err == nil {
			values, ok := value.([]any)
			if !ok || index >= len(values) {
				t.Fatalf("expected array index %s in %s", key, string(payload))
			}
			value = values[index]
			continue
		}

		next, ok := value.(map[string]any)[key]
		if !ok {
			t.Fatalf("missing key %s in %s", key, string(payload))
		}
		value = next
	}
	if value != expected {
		t.Fatalf("expected %v, got %v in %s", expected, value, string(payload))
	}
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}
