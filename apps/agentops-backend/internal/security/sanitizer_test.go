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