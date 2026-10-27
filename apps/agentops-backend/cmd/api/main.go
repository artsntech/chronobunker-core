// Package main is the entrypoint for the ChronoBunker HTTP/GraphQL API server.
// It will be expanded in Phase 1 to serve the GraphQL schema and REST health endpoints.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "chronobunker-api: starting (Phase 0 stub)")
	fmt.Fprintln(os.Stderr, "chronobunker-api: no handlers registered yet; exiting cleanly")
	os.Exit(0)
}
