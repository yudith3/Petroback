package myerror

import (
	"errors"
	"fmt"
	"testing"
)

func TestTypedErrorMatchesWrappedDomainCode(t *testing.T) {
	target := New(TypeNotFound, "REGISTRANT_NOT_FOUND", "registrant not found")
	wrapped := fmt.Errorf("lookup: %w", New(TypeNotFound, "REGISTRANT_NOT_FOUND", "different message"))
	if !errors.Is(wrapped, target) {
		t.Fatal("wrapped typed error did not match its domain code")
	}
	if errors.Is(wrapped, New(TypeConflict, "REGISTRANT_NOT_FOUND", "different type")) || errors.Is(wrapped, New(TypeNotFound, "DOCUMENT_NOT_FOUND", "different code")) {
		t.Fatal("matched a different domain error")
	}
}
