package controllers

import "errors"

// generic custom error types
var (
	ErrInvalidRequest = errors.New("invalid json")
)

// ErrorResponse is the standardized error structure which may be returned by any API
type ErrorResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"msg"`
}
