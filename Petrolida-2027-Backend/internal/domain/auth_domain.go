package domain

import (
	MyError "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
)

const (
	MESSAGE_FAILED_VERIFY_TOKEN = "failed to verify token"
	MESSAGE_USER_NOT_AUTHORIZED = "user not authorized"
	MESSAGE_API_IS_LOCKED       = "api is now locked"
)

var (
	ErrTokenInvalid    = MyError.New(MyError.TypeAuthorization, "INVALID_TOKEN", "token invalid")
	ErrTokenNotFound   = MyError.New(MyError.TypeAuthorization, "INVALID_TOKEN", "token not found")
	ErrTokenExpired    = MyError.New(MyError.TypeAuthorization, "INVALID_TOKEN", "token expired")
	ErrRoleNotAllowed  = MyError.New(MyError.TypeNotFound, "INVALID_ROLE", "role not allowed")
	ErrTokenNotAllowed = MyError.New(MyError.TypeAuthorization, "INVALID_TOKEN", "token not allowed")
)
