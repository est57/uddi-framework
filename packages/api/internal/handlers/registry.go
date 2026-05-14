package handlers

import (
	"net/http"

	"github.com/uddi-protocol/uddi/api/internal/blockchain"
	"github.com/uddi-protocol/uddi/api/internal/response"
)

type RegistryHandler struct {
	chain *blockchain.Client
}

func NewRegistryHandler(chain *blockchain.Client) *RegistryHandler {
	return &RegistryHandler{chain: chain}
}

func (h *RegistryHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.chain.RegistryStats(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "registry stats failed")
		return
	}
	response.JSON(w, http.StatusOK, stats)
}
