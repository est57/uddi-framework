package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/uddi-protocol/uddi/api/internal/middleware"
	"github.com/uddi-protocol/uddi/api/internal/response"
)

type APIKeyHandler struct {
	store middleware.APIKeyStore
}

func NewAPIKeyHandler(store middleware.APIKeyStore) *APIKeyHandler {
	return &APIKeyHandler{store: store}
}

func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID   string `json:"serviceId"`
		ServiceName string `json:"serviceName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.ServiceID = middleware.NormalizeAPIKeyInput(req.ServiceID)
	req.ServiceName = middleware.NormalizeAPIKeyInput(req.ServiceName)
	if req.ServiceID == "" {
		response.Error(w, http.StatusBadRequest, "serviceId is required")
		return
	}
	if req.ServiceName == "" {
		req.ServiceName = req.ServiceID
	}

	apiKey, err := middleware.GenerateAPIKey()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to generate API key")
		return
	}

	record, err := h.store.Create(r.Context(), req.ServiceID, req.ServiceName, apiKey)
	if err != nil {
		if errors.Is(err, middleware.ErrAPIKeyExists) {
			response.Error(w, http.StatusConflict, "API key already exists")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to create API key")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"apiKey": apiKey,
		"record": record,
	})
}

func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.List(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list API keys")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"apiKeys": records,
	})
}

func (h *APIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID string `json:"serviceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.ServiceID = middleware.NormalizeAPIKeyInput(req.ServiceID)
	if req.ServiceID == "" {
		response.Error(w, http.StatusBadRequest, "serviceId is required")
		return
	}

	if err := h.store.Revoke(r.Context(), req.ServiceID); err != nil {
		if errors.Is(err, middleware.ErrAPIKeyNotFound) {
			response.Error(w, http.StatusNotFound, "API key not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to revoke API key")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status":    "REVOKED",
		"serviceId": req.ServiceID,
	})
}
