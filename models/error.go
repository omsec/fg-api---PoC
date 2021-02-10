package models

import (
	"errors"
)

// generic custom error types
// specific errors go to the respective file of the model package
var (
	ErrNoData          = errors.New("no records found")
	ErrMultipleRecords = errors.New("mulitple records found")
	ErrGuest           = errors.New("user is guest")
	ErrNotFriend       = errors.New("user is not in friendlist")
	ErrPrivate         = errors.New("item is private")
	ErrRecordChanged   = errors.New("write conflict")
	ErrDenied          = errors.New("not allowed") // eg. upd/del not allowed
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
	ErrForzaSharingCodeTaken   = errors.New("forza sharing code already used")
)
