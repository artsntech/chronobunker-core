// Package main is the entrypoint for the ChronoBunker background worker.
// In Phase 3, this binary will host the outbox publisher and the Kafka consumer.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "chronobunker-worker: starting (Phase 0 stub)")
	fmt.Fprintln(os.Stderr, "chronobunker-worker: no workers registered yet; exiting cleanly")
	os.Exit(0)
}
