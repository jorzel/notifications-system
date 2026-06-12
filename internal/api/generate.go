// Package api holds the OpenAPI specification (openapi.yaml) — the
// source of truth for the HTTP API — and the server interface and types
// generated from it. Regenerate with `make generate`.
package api

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.yaml openapi.yaml
