package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/uddi-protocol/uddi/api/internal/blockchain"
	"github.com/uddi-protocol/uddi/api/internal/response"
)

type CredentialHandler struct {
	chain *blockchain.Client
	store CredentialStore
}

func NewCredentialHandler(chain *blockchain.Client, store CredentialStore) *CredentialHandler {
	return &CredentialHandler{chain: chain, store: store}
}

func (h *CredentialHandler) ListByDID(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.ListBySubject(r.Context(), chi.URLParam(r, "did"))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list credentials")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"did":         chi.URLParam(r, "did"),
		"credentials": records,
	})
}

func (h *CredentialHandler) Issue(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Credential map[string]any `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	record, err := credentialRecordFromPayload(req.Credential)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := h.chain.ResolveDID(r.Context(), record.SubjectDID); err != nil {
		response.Error(w, http.StatusBadRequest, "credential subject DID not found")
		return
	}
	if _, err := h.chain.ResolveDID(r.Context(), record.IssuerDID); err != nil {
		response.Error(w, http.StatusBadRequest, "credential issuer DID not found")
		return
	}

	if err := h.store.Create(r.Context(), record); err != nil {
		if errors.Is(err, ErrCredentialExists) {
			response.Error(w, http.StatusConflict, "credential already exists")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to store credential")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"status":     "ISSUED",
		"credential": record,
	})
}

func (h *CredentialHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" {
		response.Error(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.store.Revoke(r.Context(), req.ID, req.Reason); err != nil {
		if errors.Is(err, ErrCredentialNotFound) {
			response.Error(w, http.StatusNotFound, "credential not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to revoke credential")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status": "REVOKED",
		"id":     req.ID,
	})
}

func (h *CredentialHandler) Verify(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	record, err := h.store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrCredentialNotFound) {
			response.JSON(w, http.StatusOK, map[string]any{
				"id":     id,
				"valid":  false,
				"reason": "credential not found",
			})
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to verify credential")
		return
	}

	valid, reason := credentialStatus(record, time.Now().UTC())
	response.JSON(w, http.StatusOK, map[string]any{
		"id":         record.ID,
		"valid":      valid,
		"reason":     reason,
		"issuer":     record.IssuerDID,
		"subject":    record.SubjectDID,
		"types":      record.Types,
		"verifiedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func credentialRecordFromPayload(credential map[string]any) (CredentialRecord, error) {
	if credential == nil {
		return CredentialRecord{}, errors.New("credential is required")
	}

	id, ok := stringField(credential, "id")
	if !ok {
		return CredentialRecord{}, errors.New("credential.id is required")
	}

	issuer, err := issuerDID(credential["issuer"])
	if err != nil {
		return CredentialRecord{}, err
	}

	subject, ok := credential["credentialSubject"].(map[string]any)
	if !ok {
		return CredentialRecord{}, errors.New("credential.credentialSubject is required")
	}
	subjectDID, ok := stringField(subject, "id")
	if !ok {
		return CredentialRecord{}, errors.New("credential.credentialSubject.id is required")
	}

	types, err := credentialTypes(credential["type"])
	if err != nil {
		return CredentialRecord{}, err
	}

	issuanceDate, ok := stringField(credential, "issuanceDate")
	if !ok {
		return CredentialRecord{}, errors.New("credential.issuanceDate is required")
	}

	expirationDate, _ := stringField(credential, "expirationDate")
	now := time.Now().UTC().Format(time.RFC3339)
	return CredentialRecord{
		ID:             id,
		IssuerDID:      issuer,
		SubjectDID:     subjectDID,
		Types:          types,
		Credential:     credential,
		IssuanceDate:   issuanceDate,
		ExpirationDate: expirationDate,
		CreatedAt:      now,
	}, nil
}

func issuerDID(value any) (string, error) {
	if issuer, ok := value.(string); ok && issuer != "" {
		return issuer, nil
	}
	if issuer, ok := value.(map[string]any); ok {
		if id, ok := stringField(issuer, "id"); ok {
			return id, nil
		}
	}
	return "", errors.New("credential.issuer is required")
}

func credentialTypes(value any) ([]string, error) {
	rawTypes, ok := value.([]any)
	if !ok || len(rawTypes) == 0 {
		return nil, errors.New("credential.type is required")
	}

	types := make([]string, 0, len(rawTypes))
	for _, rawType := range rawTypes {
		credentialType, ok := rawType.(string)
		if !ok || credentialType == "" {
			return nil, errors.New("credential.type must contain strings")
		}
		types = append(types, credentialType)
	}
	return types, nil
}

func credentialStatus(record *CredentialRecord, now time.Time) (bool, string) {
	if record.RevokedAt != "" {
		return false, "credential revoked"
	}
	if record.ExpirationDate != "" {
		expiration, err := time.Parse(time.RFC3339, record.ExpirationDate)
		if err != nil {
			return false, "invalid credential expiration"
		}
		if !now.Before(expiration) {
			return false, "credential expired"
		}
	}
	return true, ""
}

func stringField(payload map[string]any, key string) (string, bool) {
	value, ok := payload[key].(string)
	return value, ok && value != ""
}
