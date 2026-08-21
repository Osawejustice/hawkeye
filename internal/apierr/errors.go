package apierr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is a typed application error that maps cleanly to an HTTP response.
type Error struct {
	Code    string
	Message string
	Status  int
	Err     error
	Details any
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func newError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func (e *Error) With(err error) *Error {
	clone := *e
	clone.Err = err
	return &clone
}

func (e *Error) WithMessage(message string) *Error {
	clone := *e
	clone.Message = message
	return &clone
}

func (e *Error) WithDetails(details any) *Error {
	clone := *e
	clone.Details = details
	return &clone
}

var (
	ErrInvalidInput = newError(http.StatusBadRequest, "invalid_input", "invalid request")
	ErrUnauthorized = newError(http.StatusUnauthorized, "unauthorized", "authentication required")
	ErrForbidden    = newError(http.StatusForbidden, "forbidden", "you do not have permission to perform this action")
	ErrNotFound     = newError(http.StatusNotFound, "not_found", "resource not found")
	ErrConflict     = newError(http.StatusConflict, "conflict", "resource already exists")
	ErrInternal     = newError(http.StatusInternalServerError, "internal_error", "internal server error")
	ErrBadGateway   = newError(http.StatusBadGateway, "upstream_error", "upstream service error")
	ErrUnavailable  = newError(http.StatusServiceUnavailable, "unavailable", "service unavailable")

	ErrInvalidCredentials = newError(http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	ErrInactiveUser       = newError(http.StatusForbidden, "inactive_user", "user account is disabled")
	ErrTokenExpired       = newError(http.StatusUnauthorized, "token_expired", "token has expired")
	ErrTokenInvalid       = newError(http.StatusUnauthorized, "token_invalid", "token is invalid")
	ErrTokenReused        = newError(http.StatusUnauthorized, "token_reused", "refresh token has already been used")
)

// As extracts an *Error from err, wrapping unknown errors as internal.
func As(err error) *Error {
	if err == nil {
		return nil
	}
	var app *Error
	if errors.As(err, &app) {
		return app
	}
	return ErrInternal.With(err)
}
