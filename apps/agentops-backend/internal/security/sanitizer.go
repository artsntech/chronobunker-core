// Package security provides cryptographic and output-sanitization primitives.
package security

import (
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// OutputSanitizer sanitizes LLM-generated content before it reaches the
// frontend. It removes executable HTML while preserving safe formatting.
//
// The canary is a secret token injected into the system prompt. If it
// appears in the output, the LLM has been manipulated to reveal its
// instructions. This maps to OWASP LLM07 (System Prompt Leakage).
type OutputSanitizer struct {
	policy *bluemonday.Policy
	canary string
}

// NewOutputSanitizer constructs a sanitizer with a restrictive allowlist
// policy. Only safe inline formatting is permitted. Scripts, iframes,
// objects, embeds, and event handlers are removed.
func NewOutputSanitizer(canary string) *OutputSanitizer {
	p := bluemonday.NewPolicy()

	// Allow only the HTML elements that Markdown renderers produce for
	// safe formatting. This is deliberately restrictive.
	p.AllowElements("b", "strong", "i", "em", "p", "br", "ul", "ol", "li", "code", "pre", "blockquote")

	// Allow links but only with safe schemes. bluemonday's
	// RequireParseableURLs and AllowURLSchemes enforce this at the parser
	// level, which is stronger than a regex on the href attribute.
	p.AllowAttrs("href").OnElements("a")
	p.RequireParseableURLs(true)
	p.AllowURLSchemes("http", "https", "mailto")

	// Add rel="nofollow" to external links.
	p.AddTargetBlankToFullyQualifiedLinks(true)

	return &OutputSanitizer{
		policy: p,
		canary: canary,
	}
}

// Sanitize cleans LLM output. It returns the sanitized string and a boolean
// indicating whether a canary leak was detected. If canaryLeak is true, the
// caller must abort the request and log a security event.
func (s *OutputSanitizer) Sanitize(raw string) (sanitized string, canaryLeak bool) {
	if s.canary != "" && strings.Contains(raw, s.canary) {
		return "SECURITY ERROR: response aborted due to context exploitation.", true
	}

	// bluemonday sanitizes HTML elements and attributes. Plain text and
	// Markdown syntax pass through unchanged—the frontend renders Markdown
	// with react-markdown, and the resulting HTML is sanitized again on the
	// client with DOMPurify. This is defense in depth.
	return s.policy.Sanitize(raw), false
}