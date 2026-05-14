package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/uddi-protocol/uddi/api/internal/blockchain"
	"github.com/uddi-protocol/uddi/api/internal/response"
	"github.com/uddi-protocol/uddi/api/internal/zkp"
)

type VerifyHandler struct {
	chain          *blockchain.Client
	zkp            *zkp.Service
	challengeStore ChallengeStore
}

type authChallenge struct {
	ChallengeID string `json:"challengeId"`
	Nonce       string `json:"nonce"`
	ServiceID   string `json:"serviceId"`
	ServiceName string `json:"serviceName"`
	IssuedAt    string `json:"issuedAt"`
	ExpiresAt   string `json:"expiresAt"`
	QRCode      string `json:"qrCode"`
}

type AuthVerificationResult struct {
	Valid          bool   `json:"valid"`
	DID            string `json:"did"`
	VerifiedAt     string `json:"verifiedAt"`
	VerifiedClaims []any  `json:"verifiedClaims"`
	Reason         string `json:"reason,omitempty"`
}

const (
	maxPresentationAge    = 5 * time.Minute
	maxPresentationFuture = 30 * time.Second
)

func NewVerifyHandler(chain *blockchain.Client, zkpService *zkp.Service) *VerifyHandler {
	return NewVerifyHandlerWithChallengeStore(chain, zkpService, NewMemoryChallengeStore())
}

func NewVerifyHandlerWithChallengeStore(
	chain *blockchain.Client,
	zkpService *zkp.Service,
	challengeStore ChallengeStore,
) *VerifyHandler {
	return &VerifyHandler{
		chain:          chain,
		zkp:            zkpService,
		challengeStore: challengeStore,
	}
}

func (h *VerifyHandler) CreateChallenge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID   string `json:"serviceId"`
		ServiceName string `json:"serviceName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ServiceID == "" {
		response.Error(w, http.StatusBadRequest, "serviceId is required")
		return
	}
	if headerServiceID := r.Header.Get("X-Service-ID"); headerServiceID != "" && headerServiceID != req.ServiceID {
		response.Error(w, http.StatusForbidden, "service ID mismatch")
		return
	}
	if req.ServiceName == "" {
		req.ServiceName = req.ServiceID
	}

	now := time.Now().UTC()
	challenge := authChallenge{
		ChallengeID: randomID(),
		Nonce:       randomID(),
		ServiceID:   req.ServiceID,
		ServiceName: req.ServiceName,
		IssuedAt:    now.Format(time.RFC3339),
		ExpiresAt:   now.Add(5 * time.Minute).Format(time.RFC3339),
	}
	challenge.QRCode = "uddi://auth?challengeId=" + challenge.ChallengeID

	if err := h.challengeStore.Save(r.Context(), challenge); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to store challenge")
		return
	}

	response.JSON(w, http.StatusCreated, challenge)
}

func (h *VerifyHandler) VerifyAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChallengeID  string `json:"challengeId"`
		Presentation string `json:"presentation"`
		ServiceID    string `json:"serviceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	challenge, err := h.challengeStore.Get(r.Context(), req.ChallengeID)
	if err != nil || req.Presentation == "" {
		authResult(w, false, "", "challenge not found or presentation missing")
		return
	}
	_ = h.challengeStore.Delete(r.Context(), req.ChallengeID)

	if req.ServiceID != "" && req.ServiceID != challenge.ServiceID {
		authResult(w, false, "", "service mismatch")
		return
	}

	expiresAt, _ := time.Parse(time.RFC3339, challenge.ExpiresAt)
	if !time.Now().UTC().Before(expiresAt) {
		authResult(w, false, "", "challenge expired")
		return
	}

	presentation, err := decodeAuthPresentation(req.Presentation)
	if err != nil {
		authResult(w, false, "", "invalid presentation")
		return
	}
	if presentation.ChallengeID != req.ChallengeID {
		authResult(w, false, "", "challenge mismatch")
		return
	}
	if !isPresentationTimestampValid(presentation.Timestamp, time.Now()) {
		authResult(w, false, presentation.DID, "presentation timestamp outside allowed window")
		return
	}

	didDoc, err := h.chain.ResolveDID(r.Context(), presentation.DID)
	if err != nil {
		authResult(w, false, presentation.DID, "DID not found")
		return
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(didDoc.PublicKeyBase64)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		authResult(w, false, presentation.DID, "invalid DID public key")
		return
	}

	signature, err := base64.StdEncoding.DecodeString(presentation.Signature)
	if err != nil {
		authResult(w, false, presentation.DID, "invalid signature encoding")
		return
	}

	message := []byte(req.ChallengeID + ":" + challenge.Nonce + ":" + presentation.DID + ":" + timestampString(presentation.Timestamp))
	valid := ed25519.Verify(pubKeyBytes, message, signature)
	if !valid {
		authResult(w, false, presentation.DID, "invalid signature")
		return
	}
	authResult(w, true, presentation.DID, "")
}

type authPresentation struct {
	DID         string `json:"did"`
	ChallengeID string `json:"challengeId"`
	Signature   string `json:"signature"`
	Timestamp   int64  `json:"timestamp"`
}

func decodeAuthPresentation(presentationBase64 string) (*authPresentation, error) {
	payload, err := base64.StdEncoding.DecodeString(presentationBase64)
	if err != nil {
		return nil, err
	}

	var presentation authPresentation
	if err := json.Unmarshal(payload, &presentation); err != nil {
		return nil, err
	}
	return &presentation, nil
}

func authResult(w http.ResponseWriter, valid bool, did string, reason string) {
	payload := AuthVerificationResult{
		Valid:          valid,
		DID:            did,
		VerifiedAt:     time.Now().UTC().Format(time.RFC3339),
		VerifiedClaims: []any{},
	}
	if reason != "" {
		payload.Reason = reason
	}
	response.JSON(w, http.StatusOK, payload)
}

func timestampString(timestamp int64) string {
	return strconv.FormatInt(timestamp, 10)
}

func isPresentationTimestampValid(timestamp int64, now time.Time) bool {
	presentationTime := time.UnixMilli(timestamp)
	if presentationTime.Before(now.Add(-maxPresentationAge)) {
		return false
	}
	if presentationTime.After(now.Add(maxPresentationFuture)) {
		return false
	}
	return true
}

func (h *VerifyHandler) VerifyClaim(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DID       string         `json:"did"`
		ClaimType string         `json:"claimType"`
		Proof     map[string]any `json:"proof"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	_ = h.chain
	response.JSON(w, http.StatusOK, map[string]any{
		"valid":        h.zkp.Verify(r.Context(), req.ClaimType, req.Proof),
		"claimType":    req.ClaimType,
		"verifiedAt":   time.Now().UTC().Format(time.RFC3339),
		"publicClaims": map[string]any{},
	})
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
