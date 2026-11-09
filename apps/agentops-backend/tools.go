//go:build tools

// Package tools pins development-time dependencies so that `go mod tidy`
// includes them in go.mod and go.sum. This file is excluded from the
// production build because of the `tools` build tag.
package tools

import (
	_ "github.com/99designs/gqlgen"
	_ "github.com/99designs/gqlgen/graphql/introspection"
)
