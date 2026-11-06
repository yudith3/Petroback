package myerror

import (
	"errors"
)

type Type string

const (
	TypeValidation     Type = "validation"
	TypeAuthentication Type = "authentication"
	TypeAuthorization  Type = "authorization"
	TypeNotFound       Type = "not_found"
	TypeConflict       Type = "conflict"
	TypeInternal       Type = "internal"
)

type FieldError struct {
	Field   string `json:"field"`
	Label   string `json:"label"`
	Message string `json:"message"`
	Rule    string `json:"rule,omitempty"`
}

type Error struct {
	Type    Type         `json:"-"`
	Code    string       `json:"code,omitempty"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
}

func New(errType Type, code, message string) Error {
	return Error{
		Type:    errType,
		Code:    code,
		Message: message,
	}
}

func NewValidation(fields []FieldError) Error {
	return Error{
		Type:    TypeValidation,
		Code:    "VALIDATION_ERROR",
		Message: "Input tidak valid",
		Fields:  fields,
	}
}

// NewFieldValidation membuat validation error untuk satu field saja.
func NewFieldValidation(field, label, message string) Error {
	return NewValidation([]FieldError{{Field: field, Label: label, Message: message}})
}

func ForField(err error, field, label string) error {
	var myErr Error

	if !errors.As(err, &myErr) {
		return err
	}

	myErr.Type = TypeValidation
	myErr.Code = "VALIDATION_ERROR"

	myErr.Fields = []FieldError{
		{
			Field:   field,
			Label:   label,
			Message: myErr.Message,
		},
	}

	return myErr
}

func (e Error) Error() string {
	return e.Message
}

// Error contains a slice and cannot be compared by errors.Is directly.
func (e Error) Is(target error) bool {
	other, ok := target.(Error)
	return ok && e.Code != "" && e.Type == other.Type && e.Code == other.Code
}
