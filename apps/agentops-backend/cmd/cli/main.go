// Package main is the entrypoint for the ChronoBunker administrative CLI.
// In Phase 4, this binary will expose commands like 'kill-switch', 'health-check',
// and 'verify-audit-chain'.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "chronobunker-cli: starting (Phase 0 stub)")
	fmt.Fprintln(os.Stderr, "chronobunker-cli: no commands registered yet; exiting cleanly")
	os.Exit(0)
}
