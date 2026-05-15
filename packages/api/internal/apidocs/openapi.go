package apidocs

import _ "embed"

// OpenAPIYAML is the machine-readable API contract served by the API process.
//
//go:embed openapi.yaml
var OpenAPIYAML []byte
