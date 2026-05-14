package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/uddi-protocol/uddi/api/internal/blockchain"
	"github.com/uddi-protocol/uddi/api/internal/response"
	"github.com/uddi-protocol/uddi/api/internal/zkp"
)

type ProofHandler struct {
	zkp   *zkp.Service
	chain *blockchain.Client
}

func NewProofHandler(zkpService *zkp.Service, chain *blockchain.Client) *ProofHandler {
	return &ProofHandler{zkp: zkpService, chain: chain}
}

func (h *ProofHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DID    string         `json:"did"`
		Type   string         `json:"type"`
		Params map[string]any `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	_ = h.chain
	response.JSON(w, http.StatusOK, map[string]any{
		"proof": h.zkp.Generate(r.Context(), req.Type, req.Params),
	})
}
