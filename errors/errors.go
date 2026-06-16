package errors

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Detail  interface{} `json:"detail,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func New(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func Newf(code int, format string, args ...interface{}) *AppError {
	return &AppError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func WithDetail(err *AppError, detail interface{}) *AppError {
	return &AppError{
		Code:    err.Code,
		Message: err.Message,
		Detail:  detail,
	}
}

var (
	ErrBadRequest       = New(http.StatusBadRequest, "bad request")
	ErrUnauthorized     = New(http.StatusUnauthorized, "unauthorized")
	ErrForbidden        = New(http.StatusForbidden, "forbidden")
	ErrNotFound         = New(http.StatusNotFound, "not found")
	ErrMethodNotAllowed = New(http.StatusMethodNotAllowed, "method not allowed")
	ErrInternal         = New(http.StatusInternalServerError, "internal server error")
	ErrValidation       = New(http.StatusUnprocessableEntity, "validation failed")
)

func IsAppError(err error) (*AppError, bool) {
	if appErr, ok := err.(*AppError); ok {
		return appErr, true
	}
	return nil, false
}

func FromError(err error) *AppError {
	if appErr, ok := IsAppError(err); ok {
		return appErr
	}
	return &AppError{
		Code:    http.StatusInternalServerError,
		Message: err.Error(),
	}
}
