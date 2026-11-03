// Package api is the blog's HTTP contract as oapi-codegen generates it
// from openapi.yaml: the models, the strict server interface over
// net/http, the typed client and the embedded document.
package api

//go:generate go tool oapi-codegen -config config.yaml ../openapi.yaml
