package internal_errors

import "errors"

var (
	ErrUnsupportedFormat = errors.New("unsupported image format")
	ErrTooManyRequests   = errors.New("too many concurrent requests")
	ErrQueueFull         = errors.New("queue full, try later")
)

type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func NewValidationError(msg string) error { return &ValidationError{Msg: msg} }
