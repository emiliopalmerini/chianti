package apperror_test

import (
	"errors"
	"testing"

	"github.com/emiliopalmerini/chianti/kernel/apperror"
)

func TestNotFound(t *testing.T) {
	err := apperror.NotFound("event", "42")
	if !apperror.Is(err, apperror.KindNotFound) {
		t.Fatalf("expected NotFound kind")
	}
	if err.Detail != "42" {
		t.Errorf("expected detail '42', got %q", err.Detail)
	}
}

func TestValidationFields(t *testing.T) {
	err := apperror.Validation("campo mancante", map[string]string{"title": "obbligatorio"})
	fields := apperror.FieldsOf(err)
	if fields["title"] != "obbligatorio" {
		t.Fatalf("expected field error, got %v", fields)
	}
}

func TestIsWithWrapped(t *testing.T) {
	inner := apperror.Conflict("duplicato")
	wrapped := errors.Join(errors.New("context"), inner)
	if !apperror.Is(wrapped, apperror.KindConflict) {
		t.Fatal("expected Is to detect wrapped conflict")
	}
}

func TestInternalPreservesUnderlying(t *testing.T) {
	base := errors.New("boom")
	err := apperror.Internal(base)
	if !errors.Is(err, base) {
		t.Fatal("expected errors.Is to find wrapped error")
	}
}

func TestKindString(t *testing.T) {
	cases := map[apperror.Kind]string{
		apperror.KindInternal:     "internal",
		apperror.KindNotFound:     "not_found",
		apperror.KindValidation:   "validation",
		apperror.KindConflict:     "conflict",
		apperror.KindUnauthorized: "unauthorized",
		apperror.KindForbidden:    "forbidden",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}
