package domain

import "errors"

var (
	ErrNotFound             = errors.New("not found")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrForbidden            = errors.New("forbidden")
	ErrConflict             = errors.New("conflict")
	ErrInvalidPassword      = errors.New("invalid password")
	ErrInsufficientBalance  = errors.New("insufficient scan balance")
)
