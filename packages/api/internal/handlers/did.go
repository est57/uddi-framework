// Package handlers contains HTTP handlers for UDDI API resources.
package handlers

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/uddi-protocol/uddi/api/internal/blockchain"
	"github.com/uddi-protocol/uddi/api/internal/response"
)

type DIDHandler struct {
	chain *blockchain.Client
}

func NewDIDHandler(chain *blockchain.Client) *DIDHandler {
	return &DIDHandler{chain: chain}
}

type RegisterDIDRequest struct {
	DID             string `json:"did"`
	PublicKeyBase64 string `json:"publicKeyBase64"`
	SignatureBase64 string `json:"signatureBase64"`
	Timestamp       string `json:"timestamp"`
}

type RegisterDIDResponse struct {
	DID       string `json:"did"`
	TxHash    string `json:"txHash"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

func (h *DIDHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterDIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !isValidDID(req.DID) {
		response.Error(w, http.StatusBadRequest, "invalid DID format")
		return
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(req.PublicKeyBase64)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		response.Error(w, http.StatusBadRequest, "invalid public key")
		return
	}

	challenge := []byte("register:" + req.DID + ":" + req.Timestamp)
	sigBytes, err := base64.StdEncoding.DecodeString(req.SignatureBase64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}

	if !ed25519.Verify(pubKeyBytes, challenge, sigBytes) {
		response.Error(w, http.StatusUnauthorized, "signature verification failed")
		return
	}

	exists, err := h.chain.DIDExists(r.Context(), req.DID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "blockchain query failed")
		return
	}
	if exists {
		response.Error(w, http.StatusConflict, "DID already registered")
		return
	}

	txHash, err := h.chain.RegisterDID(r.Context(), blockchain.RegisterDIDParams{
		DID:       req.DID,
		PublicKey: pubKeyBytes,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to register DID on blockchain")
		return
	}

	response.JSON(w, http.StatusCreated, RegisterDIDResponse{
		DID:       req.DID,
		TxHash:    txHash,
		Status:    "REGISTERED",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *DIDHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	did := chi.URLParam(r, "did")

	if !isValidDID(did) {
		response.Error(w, http.StatusBadRequest, "invalid DID format")
		return
	}

	didDoc, err := h.chain.ResolveDID(r.Context(), did)
	if err != nil {
		if err == blockchain.ErrDIDNotFound {
			response.Error(w, http.StatusNotFound, "DID not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "resolution failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"didDocument": didDoc,
		"didDocumentMetadata": map[string]any{
			"created":     didDoc.Created,
			"updated":     didDoc.Updated,
			"deactivated": didDoc.Deactivated,
		},
		"didResolutionMetadata": map[string]any{
			"contentType": "application/did+ld+json",
		},
	})
}

type RevokeDIDRequest struct {
	DID             string `json:"did"`
	SignatureBase64 string `json:"signatureBase64"`
	Timestamp       string `json:"timestamp"`
}

func (h *DIDHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	var req RevokeDIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	didDoc, err := h.chain.ResolveDID(r.Context(), req.DID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "DID not found")
		return
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(didDoc.PublicKeyBase64)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "invalid stored public key")
		return
	}

	challenge := []byte("revoke:" + req.DID + ":" + req.Timestamp)
	sigBytes, err := base64.StdEncoding.DecodeString(req.SignatureBase64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}

	if !ed25519.Verify(pubKeyBytes, challenge, sigBytes) {
		response.Error(w, http.StatusUnauthorized, "signature verification failed")
		return
	}

	if err := h.chain.RevokeDID(r.Context(), req.DID); err != nil {
		response.Error(w, http.StatusInternalServerError, "revocation failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status": "REVOKED",
		"did":    req.DID,
	})
}

func (h *DIDHandler) Update(w http.ResponseWriter, r *http.Request) {
	did := chi.URLParam(r, "did")
	var req struct {
		DID             string   `json:"did"`
		PublicKeyBase64 string   `json:"publicKeyBase64"`
		Context         []string `json:"context"`
		SignatureBase64 string   `json:"signatureBase64"`
		Timestamp       string   `json:"timestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DID == "" {
		req.DID = did
	}
	if did != req.DID {
		response.Error(w, http.StatusBadRequest, "DID path and body mismatch")
		return
	}
	if !isValidDID(req.DID) {
		response.Error(w, http.StatusBadRequest, "invalid DID format")
		return
	}

	didDoc, err := h.chain.ResolveDID(r.Context(), req.DID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "DID not found")
		return
	}
	if didDoc.Deactivated {
		response.Error(w, http.StatusConflict, "DID is deactivated")
		return
	}

	currentPubKeyBytes, err := base64.StdEncoding.DecodeString(didDoc.PublicKeyBase64)
	if err != nil || len(currentPubKeyBytes) != ed25519.PublicKeySize {
		response.Error(w, http.StatusInternalServerError, "invalid stored public key")
		return
	}
	newPubKeyBytes, err := base64.StdEncoding.DecodeString(req.PublicKeyBase64)
	if err != nil || len(newPubKeyBytes) != ed25519.PublicKeySize {
		response.Error(w, http.StatusBadRequest, "invalid public key")
		return
	}

	challenge := []byte("update:" + req.DID + ":" + req.PublicKeyBase64 + ":" + req.Timestamp)
	sigBytes, err := base64.StdEncoding.DecodeString(req.SignatureBase64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}
	if !ed25519.Verify(currentPubKeyBytes, challenge, sigBytes) {
		response.Error(w, http.StatusUnauthorized, "signature verification failed")
		return
	}

	txHash, err := h.chain.UpdateDID(r.Context(), blockchain.UpdateDIDParams{
		DID:       req.DID,
		PublicKey: newPubKeyBytes,
		Context:   req.Context,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "DID update failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"did":       req.DID,
		"txHash":    txHash,
		"status":    "UPDATED",
		"updatedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func isValidDID(did string) bool {
	if !strings.HasPrefix(did, "did:uddi:z") {
		return false
	}
	suffix := did[len("did:uddi:z"):]
	return len(suffix) >= 40 && len(suffix) <= 100
}
