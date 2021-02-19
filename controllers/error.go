package controllers

import (
	"errors"
	"fmt"
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

	fmt.Println(err)
	switch err {
	// system
	case models.ErrMultipleRecords:
		apiError.Code = MultipleRecords
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusInternalServerError
	case models.ErrRecordChanged:
		apiError.Code = RecordChanged
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusInternalServerError // ToDO: evtl. was anderes?
	// permissions
	case models.ErrGuest:
		apiError.Code = PermissionGuest
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusUnprocessableEntity
	case models.ErrNotFriend:
		apiError.Code = PermissionNotShared
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusUnprocessableEntity
	case models.ErrPrivate:
		apiError.Code = PermissionPrivate
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusUnprocessableEntity
	case models.ErrDenied:
		apiError.Code = ActionDenied
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusUnprocessableEntity
		// user
	case models.ErrUserNameNotAvailable:
		apiError.Code = UserNameTaken
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusUnprocessableEntity
	case models.ErrEMailAddressTaken:
		apiError.Code = EMailAddressTaken
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusUnprocessableEntity
	case models.ErrInvalidUser:
		apiError.Code = InvalidRequest
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusUnprocessableEntity
	case models.ErrInvalidPassword:
		apiError.Code = InvalidPassword
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusUnprocessableEntity
	// course
	case models.ErrCourseNameMissing:
		apiError.Code = CourseNameMissing
		apiError.Message = apiError.String(apiError.Code)
		httpStatus = http.StatusUnprocessableEntity
	case models.ErrForzaSharingCodeTaken:
		apiError.Code = ForzaShareTaken
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
	InvalidLogin
	// generic system
	MultipleRecords
	RecordChanged
	ActionDenied
	// permission
	PermissionGuest
	PermissionNotShared
	PermissionPrivate
	// user
	UserNameTaken
	EMailAddressTaken
	InvalidPassword
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
	case InvalidLogin:
		msg = "invalid user name or password"
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
