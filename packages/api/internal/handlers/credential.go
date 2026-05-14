package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/uddi-protocol/uddi/api/internal/blockchain"
	"github.com/uddi-protocol/uddi/api/internal/response"
)

type CredentialHandler struct {
	chain *blockchain.Client
}

func NewCredentialHandler(chain *blockchain.Client) *CredentialHandler {
	return &CredentialHandler{chain: chain}
}

func (h *CredentialHandler) ListByDID(w http.ResponseWriter, r *http.Request) {
	_ = h.chain
	response.JSON(w, http.StatusOK, map[string]any{
		"did":         chi.URLParam(r, "did"),
		"credentials": []any{},
	})
}

func (h *CredentialHandler) Issue(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusAccepted, map[string]string{
		"status": "PENDING_IMPLEMENTATION",
	})
}

func (h *CredentialHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusAccepted, map[string]string{
		"status": "PENDING_IMPLEMENTATION",
	})
}

func (h *CredentialHandler) Verify(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]any{
		"id":     chi.URLParam(r, "id"),
		"valid":  false,
		"reason": "credential registry not implemented yet",
	})
}
