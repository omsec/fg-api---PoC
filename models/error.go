package models

import (
	"errors"
)

// custom error types (generic types found in apperror package)

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

// comment
// transformed by controllers to respective Unprocessable Entity (422)
var (
	ErrCommentEmpty = errors.New("comment is required")
)
