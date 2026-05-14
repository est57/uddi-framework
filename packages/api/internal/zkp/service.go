package zkp

import "context"

type Service struct {
	url string
}

func NewService(url string) *Service {
	return &Service{url: url}
}

func (s *Service) Generate(_ context.Context, proofType string, params map[string]any) map[string]any {
	return map[string]any{
		"protocol":      "groth16",
		"curve":         "bn128",
		"type":          proofType,
		"params":        params,
		"serviceUrl":    s.url,
		"proof":         map[string]any{"pi_a": []string{}, "pi_b": [][]string{}, "pi_c": []string{}},
		"publicSignals": []string{},
	}
}

func (s *Service) Verify(_ context.Context, proofType string, proof map[string]any) bool {
	return proofType != "" && proof != nil
}
