package zkp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Service struct {
	url    string
	client *http.Client
}

type VerificationResult struct {
	Valid        bool           `json:"valid"`
	Reason       string         `json:"reason,omitempty"`
	PublicClaims map[string]any `json:"publicClaims"`
}

func NewService(url string) *Service {
	return &Service{
		url: strings.TrimSpace(url),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *Service) Generate(ctx context.Context, proofType string, params map[string]any) (map[string]any, error) {
	proofType = strings.TrimSpace(proofType)
	if proofType == "" {
		return nil, errors.New("proof type is required")
	}
	if params == nil {
		params = map[string]any{}
	}
	if s.isRemote() {
		return s.remoteGenerate(ctx, proofType, params)
	}

	return map[string]any{
		"protocol":      "groth16",
		"curve":         "bn128",
		"type":          proofType,
		"circuit":       circuitName(proofType),
		"mode":          "development",
		"params":        params,
		"serviceUrl":    s.url,
		"proof":         map[string]any{"pi_a": []string{}, "pi_b": [][]string{}, "pi_c": []string{}},
		"publicSignals": []string{},
	}, nil
}

func (s *Service) Verify(ctx context.Context, proofType string, proof map[string]any) VerificationResult {
	proofType = strings.TrimSpace(proofType)
	if proofType == "" {
		return invalid("claim type is required")
	}
	if proof == nil {
		return invalid("proof is required")
	}
	if s.isRemote() {
		return s.remoteVerify(ctx, proofType, proof)
	}
	if reason := validateGroth16Proof(proofType, proof); reason != "" {
		return invalid(reason)
	}
	return VerificationResult{
		Valid:        true,
		PublicClaims: publicClaims(proof),
	}
}

func (s *Service) isRemote() bool {
	return strings.HasPrefix(s.url, "http://") || strings.HasPrefix(s.url, "https://")
}

func (s *Service) remoteGenerate(ctx context.Context, proofType string, params map[string]any) (map[string]any, error) {
	var payload struct {
		Proof map[string]any `json:"proof"`
	}
	err := s.postJSON(ctx, "/generate", map[string]any{
		"type":   proofType,
		"params": params,
	}, &payload)
	if err != nil {
		return nil, err
	}
	if payload.Proof == nil {
		return nil, errors.New("zkp service returned no proof")
	}
	return payload.Proof, nil
}

func (s *Service) remoteVerify(ctx context.Context, proofType string, proof map[string]any) VerificationResult {
	var payload VerificationResult
	err := s.postJSON(ctx, "/verify", map[string]any{
		"type":  proofType,
		"proof": proof,
	}, &payload)
	if err != nil {
		return invalid(err.Error())
	}
	if payload.PublicClaims == nil {
		payload.PublicClaims = map[string]any{}
	}
	if !payload.Valid && payload.Reason == "" {
		payload.Reason = "invalid proof"
	}
	return payload
}

func (s *Service) postJSON(ctx context.Context, path string, body any, out any) error {
	baseURL, err := url.Parse(s.url)
	if err != nil {
		return fmt.Errorf("invalid zkp service url: %w", err)
	}
	endpoint := baseURL.ResolveReference(&url.URL{Path: strings.TrimRight(baseURL.Path, "/") + path})

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("zkp service request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("zkp service returned status %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("decode zkp service response: %w", err)
	}
	return nil
}

func validateGroth16Proof(proofType string, proof map[string]any) string {
	if stringValue(proof, "type") != proofType {
		return "proof type does not match claim type"
	}
	if stringValue(proof, "protocol") != "groth16" {
		return "unsupported proof protocol"
	}
	if stringValue(proof, "curve") != "bn128" {
		return "unsupported proof curve"
	}
	proofBody, ok := proof["proof"].(map[string]any)
	if !ok {
		return "proof body is required"
	}
	if _, ok := proofBody["pi_a"]; !ok {
		return "proof pi_a is required"
	}
	if _, ok := proofBody["pi_b"]; !ok {
		return "proof pi_b is required"
	}
	if _, ok := proofBody["pi_c"]; !ok {
		return "proof pi_c is required"
	}
	if !isSlice(proof["publicSignals"]) {
		return "publicSignals is required"
	}
	return ""
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func publicClaims(proof map[string]any) map[string]any {
	claims, ok := proof["publicClaims"].(map[string]any)
	if !ok || claims == nil {
		return map[string]any{}
	}
	return claims
}

func isSlice(value any) bool {
	switch value.(type) {
	case []any, []string, []int, []float64:
		return true
	default:
		return false
	}
}

func invalid(reason string) VerificationResult {
	return VerificationResult{
		Valid:        false,
		Reason:       reason,
		PublicClaims: map[string]any{},
	}
}

func circuitName(proofType string) string {
	switch proofType {
	case "age", "age_verification":
		return "age_verification"
	case "citizenship", "citizenship_verification":
		return "citizenship_verification"
	default:
		return proofType
	}
}
