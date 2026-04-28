// Package apperror provides a typed error wrapper carrying a kind,
// user-safe message, internal detail, and optional field-level errors.
// HTTP translation is a caller concern.
package apperror

import (
	"errors"
	"fmt"
)

type Kind int

const (
	KindInternal Kind = iota
	KindNotFound
	KindValidation
	KindConflict
	KindUnauthorized
	KindForbidden
)

func (k Kind) String() string {
	switch k {
	case KindNotFound:
		return "not_found"
	case KindValidation:
		return "validation"
	case KindConflict:
		return "conflict"
	case KindUnauthorized:
		return "unauthorized"
	case KindForbidden:
		return "forbidden"
	default:
		return "internal"
	}
}

type Error struct {
	Kind    Kind
	Msg     string
	Detail  string
	Fields  map[string]string
	wrapped error
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Kind, e.Msg, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Msg)
}

func (e *Error) Unwrap() error { return e.wrapped }

func NotFound(resource, id string) *Error {
	return &Error{
		Kind:   KindNotFound,
		Msg:    fmt.Sprintf("%s non trovato", resource),
		Detail: id,
	}
}

func Validation(msg string, fields map[string]string) *Error {
	return &Error{Kind: KindValidation, Msg: msg, Fields: fields}
}

func Conflict(msg string) *Error {
	return &Error{Kind: KindConflict, Msg: msg}
}

func Unauthorized(msg string) *Error {
	return &Error{Kind: KindUnauthorized, Msg: msg}
}

func Forbidden(msg string) *Error {
	return &Error{Kind: KindForbidden, Msg: msg}
}

func Internal(err error) *Error {
	return &Error{Kind: KindInternal, Msg: "errore interno", Detail: err.Error(), wrapped: err}
}

func Is(err error, kind Kind) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind == kind
	}
	return false
}

func FieldsOf(err error) map[string]string {
	var e *Error
	if errors.As(err, &e) {
		return e.Fields
	}
	return nil
}
