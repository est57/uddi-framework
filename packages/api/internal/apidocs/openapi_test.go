package apidocs

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedOpenAPIYAMLSync(t *testing.T) {
	rootSpecPath := filepath.Join("..", "..", "..", "..", "docs", "openapi.yaml")
	rootSpec, err := os.ReadFile(rootSpecPath)
	if err != nil {
		t.Fatalf("read root OpenAPI spec: %v", err)
	}
	if !bytes.Equal(OpenAPIYAML, rootSpec) {
		t.Fatalf("embedded OpenAPI spec is out of sync with docs/openapi.yaml")
	}
}
