package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAPIKey(t *testing.T) {
	store := NewMemoryAPIKeyStore()
	next := RequireAPIKey(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	tests := []struct {
		name       string
		apiKey     string
		serviceID  string
		wantStatus int
	}{
		{name: "valid", apiKey: "dev-api-key", serviceID: "dev-service", wantStatus: http.StatusNoContent},
		{name: "missing key", serviceID: "dev-service", wantStatus: http.StatusUnauthorized},
		{name: "missing service", apiKey: "dev-api-key", wantStatus: http.StatusUnauthorized},
		{name: "invalid", apiKey: "wrong-key", serviceID: "dev-service", wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.apiKey != "" {
				req.Header.Set("X-API-Key", tt.apiKey)
			}
			if tt.serviceID != "" {
				req.Header.Set("X-Service-ID", tt.serviceID)
			}

			res := httptest.NewRecorder()
			next.ServeHTTP(res, req)

			if res.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, res.Code)
			}
		})
	}
}
