package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyLimitRejectsOversizedRequest(t *testing.T) {
	// Build a handler that wraps the body with MaxBytesReader, exactly as
	// the production server does.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<10) // 1 KiB for the test
		// Attempt to read the full body.
		_, err := bytes.NewBuffer(nil).ReadFrom(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	oversized := strings.Repeat("a", 2<<10) // 2 KiB, over the 1 KiB limit
	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(oversized))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rec.Code)
	}
}
