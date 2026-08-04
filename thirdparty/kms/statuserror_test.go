// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.
package kms

import (
	"errors"
	"fmt"
	"testing"
)

// TestStatusError_ErrorsAsUnwraps proves a KMS StatusError is recoverable via
// errors.As even through a %w wrap — the property that lets Hydrate branch on
// the status code instead of string-matching the message.
func TestStatusError_ErrorsAsUnwraps(t *testing.T) {
	base := &StatusError{Op: "get secret", Code: 404, Body: "not found"}
	wrapped := fmt.Errorf("hydrate acme/stripe: %w", base)

	var se *StatusError
	if !errors.As(wrapped, &se) {
		t.Fatal("errors.As must recover *StatusError through a %w wrap")
	}
	if se.Code != 404 {
		t.Fatalf("recovered Code = %d, want 404", se.Code)
	}
}

// TestStatusError_Message pins the human-readable format (unchanged from the
// prior fmt.Errorf string, so logs/dashboards are unaffected).
func TestStatusError_Message(t *testing.T) {
	e := &StatusError{Op: "get secret", Code: 404, Body: "missing"}
	if got, want := e.Error(), "kms get secret failed (status 404): missing"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
