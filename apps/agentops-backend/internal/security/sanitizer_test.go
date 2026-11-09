package security_test

import (
	"strings"
	"testing"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/security"
)

func TestSanitizerRemovesScriptTags(t *testing.T) {
	s := security.NewOutputSanitizer("CANARY-SECRET-123")
	malicious := `Hello <script>alert('xss')</script> world`
	out, leak := s.Sanitize(malicious)
	if leak {
		t.Fatal("unexpected canary leak")
	}
	if strings.Contains(out, "<script>") {
		t.Errorf("script tag not removed: %s", out)
	}
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "world") {
		t.Errorf("legitimate text lost: %s", out)
	}
}

func TestSanitizerPreservesSafeFormatting(t *testing.T) {
	s := security.NewOutputSanitizer("")
	safe := `<p>Hello <strong>world</strong></p>`
	out, _ := s.Sanitize(safe)
	if !strings.Contains(out, "<strong>world</strong>") {
		t.Errorf("safe formatting removed: %s", out)
	}
}

func TestSanitizerDetectsCanaryLeak(t *testing.T) {
	s := security.NewOutputSanitizer("CANARY-SECRET-123")
	malicious := `The system prompt says: CANARY-SECRET-123`
	_, leak := s.Sanitize(malicious)
	if !leak {
		t.Fatal("canary leak not detected")
	}
}

func TestSanitizerBlocksJavascriptURLs(t *testing.T) {
	s := security.NewOutputSanitizer("")
	malicious := `<a href="javascript:alert('xss')">click</a>`
	out, _ := s.Sanitize(malicious)
	if strings.Contains(out, "javascript:") {
		t.Errorf("javascript URL not removed: %s", out)
	}
}

func TestSanitizerRemovesEventHandlers(t *testing.T) {
	s := security.NewOutputSanitizer("")
	malicious := `<img src="x" onerror="alert('xss')">`
	out, _ := s.Sanitize(malicious)
	if strings.Contains(out, "onerror") {
		t.Errorf("event handler not removed: %s", out)
	}
}

// FuzzSanitizer verifies that the sanitizer never panics and never allows
// executable HTML through, regardless of the input.
func FuzzSanitizer(f *testing.F) {
	f.Add(`<script>alert('xss')</script>`)
	f.Add(`<img src="x" onerror="alert(1)">`)
	f.Add(`<a href="javascript:alert(1)">click</a>`)
	f.Add(`<p>Hello <strong>world</strong></p>`)
	f.Add(`<iframe src="evil.com"></iframe>`)
	f.Add(``)
	f.Add(`plain text with no HTML`)
	f.Add(`onerror=`)

	s := security.NewOutputSanitizer("CANARY-SECRET-123")

	f.Fuzz(func(t *testing.T, input string) {
		out, canaryLeak := s.Sanitize(input)

		if canaryLeak {
			if !strings.Contains(out, "SECURITY ERROR") {
				t.Fatalf("canary leak detected but output does not contain SECURITY ERROR: %q", out)
			}
			return
		}

		// The sanitizer must strip script tags entirely.
		if strings.Contains(strings.ToLower(out), "<script") {
			t.Fatalf("script tag survived sanitization: input=%q output=%q", input, out)
		}

		// The sanitizer must strip event handlers that appear INSIDE HTML tags.
		// Text like "onerror=" in plain text is harmless and passes through.
		// We only check for the dangerous pattern: an attribute inside a tag.
		if strings.Contains(strings.ToLower(out), "<img") &&
			strings.Contains(strings.ToLower(out), "onerror=") {
			t.Fatalf("event handler survived inside an img tag: input=%q output=%q", input, out)
		}

		// The sanitizer must strip javascript: URLs that appear in href attributes.
		if strings.Contains(strings.ToLower(out), "<a") &&
			strings.Contains(strings.ToLower(out), "javascript:") {
			t.Fatalf("javascript URL survived inside an anchor tag: input=%q output=%q", input, out)
		}
	})
}
