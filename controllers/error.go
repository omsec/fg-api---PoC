package controllers

import (
	"errors"
	"forza-garage/models"
	"net/http"
)

// generic custom error types
var (
	ErrInvalidRequest = errors.New("invalid json")
)

// ErrorResponse is the standardized error structure which may be returned by any API
type ErrorResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"msg"`
}

// HandleError encodes the std ErrorResponse
func HandleError(err error) (httpStatus int, apiError ErrorResponse) {

	if err == nil {
		apiError.Code = 0
		apiError.Message = ""

		return 0, apiError
	}

	switch err {
	case models.ErrCourseNameMissing:
		apiError.Code = CourseNameMissing
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusUnprocessableEntity
	default:
		apiError.Code = SystemError
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusInternalServerError
	}
	return httpStatus, apiError
}

// Application Error Codes (API Errors)
const (
	// client/api
	InvalidJSON int32 = (10000 + iota)
	InvalidRequest
	// generic system
	MultipleRecords
	RecordChanged
	ActionDenied
	// permission
	PermissionGuest
	PermissionNotShared
	PermissionPrivate
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
	// common (system)
	case InvalidJSON:
		msg = "Invalid JSON"
	case InvalidRequest:
		msg = "Invalid Request" // JSON was correct, data was not
	case MultipleRecords:
		msg = "multiple records found"
	case RecordChanged:
		msg = "record changed by another user"
	case ActionDenied:
		msg = "update/delete action not allowed"
	// permision (item access)
	case PermissionGuest:
		msg = "user is guest"
	case PermissionNotShared:
		msg = "item is not shared" // user is not friends with creator
	case PermissionPrivate:
		msg = "item is private"
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
