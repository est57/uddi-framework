package zkp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestLocalGenerateAndVerify(t *testing.T) {
	service := NewService("memory://zkp")

	proof, err := service.Generate(context.Background(), "age", map[string]any{"minimumAge": 18})
	if err != nil {
		t.Fatalf("generate proof: %v", err)
	}
	if proof["type"] != "age" {
		t.Fatalf("expected proof type age, got %v", proof["type"])
	}
	if proof["circuit"] != "age_verification" {
		t.Fatalf("expected age circuit, got %v", proof["circuit"])
	}

	result := service.Verify(context.Background(), "age", proof)
	if !result.Valid {
		t.Fatalf("expected generated local proof to verify, got reason %q", result.Reason)
	}
}

func TestLocalVerifyRejectsMalformedProof(t *testing.T) {
	service := NewService("memory://zkp")

	result := service.Verify(context.Background(), "age", map[string]any{
		"protocol": "groth16",
		"curve":    "bn128",
		"type":     "citizenship",
		"proof":    map[string]any{},
	})

	if result.Valid {
		t.Fatalf("expected malformed proof to be invalid")
	}
	if result.Reason != "proof type does not match claim type" {
		t.Fatalf("unexpected reason: %q", result.Reason)
	}
}

func TestRemoteGenerateAndVerify(t *testing.T) {
	var generateCalled bool
	var verifyCalled bool

	service := NewService("http://zkp.test")
	service.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			var response map[string]any
			status := http.StatusOK

			switch r.URL.Path {
			case "/generate":
				generateCalled = true
				response = map[string]any{
					"proof": map[string]any{
						"protocol":      "groth16",
						"curve":         "bn128",
						"type":          "age",
						"proof":         map[string]any{"pi_a": []string{}, "pi_b": [][]string{}, "pi_c": []string{}},
						"publicSignals": []string{},
					},
				}
			case "/verify":
				verifyCalled = true
				response = map[string]any{
					"valid":        true,
					"publicClaims": map[string]any{"minimumAge": 18},
				}
			default:
				status = http.StatusNotFound
				response = map[string]any{"error": "not found"}
			}

			body, err := json.Marshal(response)
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: status,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(body)),
				Request:    r,
			}, nil
		}),
	}

	proof, err := service.Generate(context.Background(), "age", map[string]any{"minimumAge": 18})
	if err != nil {
		t.Fatalf("remote generate: %v", err)
	}
	result := service.Verify(context.Background(), "age", proof)

	if !generateCalled || !verifyCalled {
		t.Fatalf("expected generate and verify endpoints to be called")
	}
	if !result.Valid {
		t.Fatalf("expected remote verification to be valid: %q", result.Reason)
	}
	if result.PublicClaims["minimumAge"] != float64(18) {
		t.Fatalf("expected public claim minimumAge 18, got %v", result.PublicClaims["minimumAge"])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
