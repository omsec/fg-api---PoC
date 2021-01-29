package models

import (
	"errors"
)

// generic custom error types
// specific errors go to the respective file of the model package
var (
	ErrNoData          = errors.New("no records found")
	ErrMultipleRecords = errors.New("mulitple records found")
)

// custom error types

// user
// custom error types - evtl in eigenes file
var (
	ErrUserNameNotAvailable = errors.New("user name is not available")
	ErrEMailAddressTaken    = errors.New("email-address is already used")
	ErrInvalidUser          = errors.New("invalid user name or password")
	ErrInvalidPassword      = errors.New("password does not meet rules")
	ErrInvalidFriend        = errors.New("could not add/remove friend")
)

// course
// transformed by controllers to respective Unprocessable Entity (422)
var (
	ErrForzaSharingCodeMissing = errors.New("sharing code is required")
	ErrCourseNameMissing       = errors.New("course name is required")
	ErrSeriesMissing           = errors.New("series is required")
	ErrForzaSharingCodeTaken   = errors.New("forza sharing code already used")
)
