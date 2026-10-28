package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// schemaCache caches compiled JSON schemas by tool name so that repeated
// invocations do not recompile the schema on every request.
type schemaCache struct {
	mu      sync.RWMutex
	compiled map[string]*jsonschema.Schema
}

var globalSchemaCache = &schemaCache{compiled: make(map[string]*jsonschema.Schema)}

// validateSchema validates params against the tool's JSON Schema.
// The schema is compiled once and cached for subsequent calls.
func validateSchema(rawSchema []byte, params map[string]any) error {
	// Use the schema hash as cache key to handle schema updates.
	key := string(rawSchema)

	globalSchemaCache.mu.RLock()
	cached := globalSchemaCache.compiled[key]
	globalSchemaCache.mu.RUnlock()

	if cached == nil {
		compiler := jsonschema.NewCompiler()
		if err := compiler.AddResource("inline.json", bytes.NewReader(rawSchema)); err != nil {
			return fmt.Errorf("compile schema: %w", err)
		}
		compiled, err := compiler.Compile("inline.json")
		if err != nil {
			return fmt.Errorf("compile schema: %w", err)
		}
		globalSchemaCache.mu.Lock()
		globalSchemaCache.compiled[key] = compiled
		globalSchemaCache.mu.Unlock()
		cached = compiled
	}

	// Normalize params via JSON round-trip to ensure types match what the
	// schema expects (e.g. json.Number vs float64).
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}
	var normalized any
	if err := json.Unmarshal(paramsJSON, &normalized); err != nil {
		return fmt.Errorf("unmarshal params: %w", err)
	}

	if err := cached.Validate(normalized); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	return nil
}