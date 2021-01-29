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

// Application Error Codes (API Errors)
const (
	InvalidJSON int32 = (10000 + iota)
	InvalidRequest
	// user
	InvalidFriend
	// course
	CourseNameMissing
	ForzaShareTaken
	SystemError = 99999
)

func (er ErrorResponse) String(code int32) string {
	msg := ""
	switch code {
	// common
	case InvalidJSON:
		msg = "Invalid JSON"
	case InvalidRequest:
		msg = "Invalid Request" // JSON was correct, data was not
	// user
	case InvalidFriend:
		msg = "could not add or remove friend"
	// course
	case CourseNameMissing:
		msg = "course name is required"
	case ForzaShareTaken:
		msg = "Duplicate Forza Share Code"
	case SystemError:
		msg = "Server Problem"
	}

	return msg
}
