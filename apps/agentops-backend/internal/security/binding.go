package security

import (
	"fmt"
	"slices"
)

// checkResourceBinding ensures that every resource ID referenced in the
// params is present in the token's allowed ResourceIDs list.
//
// This prevents prompt injection attacks where a compromised or manipulated
// LLM tries to invoke a tool on a resource the user does not own.
func checkResourceBinding(tok *CapabilityToken, params map[string]any) error {
	// Known resource identifier field names in tool parameters.
	// Add new ones here as the tool registry grows.
	resourceFields := []string{"order_id", "customer_id", "ticket_id"}

	for _, field := range resourceFields {
		raw, ok := params[field]
		if !ok {
			continue
		}
		id, ok := raw.(string)
		if !ok {
			return fmt.Errorf("binding: %s must be a string", field)
		}
		if !slices.Contains(tok.ResourceIDs, id) {
			return fmt.Errorf("binding: %s=%q not in session scope", field, id)
		}
	}
	return nil
}
