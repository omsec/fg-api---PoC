package models

import "errors"

// generic custom error types
// specific errors go to the respective file of the model package
var (
	ErrNoData = errors.New("no records found")
)
