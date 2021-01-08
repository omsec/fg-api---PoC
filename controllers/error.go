package controllers

import "errors"

// generic custom error types
var (
	ErrInvalidRequest = errors.New("invalid json")
)
