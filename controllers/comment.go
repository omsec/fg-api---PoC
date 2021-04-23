package controllers

import (
	"forza-garage/apperror"
	"forza-garage/authentication"
	"forza-garage/environment"
	"forza-garage/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AddComment creates a new comment or reply
// client soll Profile Type mitgeben, dann braucht's nur einen Handler
func AddComment(c *gin.Context) {

	var (
		err      error
		data     models.Comment
		apiError ErrorResponse
	)

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// use "shouldBind" not all fields are required in this context
	if err = c.ShouldBindJSON(&data); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}
	// validate request
	comment, err := environment.Env.CommentModel.Validate(data)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	// userID als Parameter, damit hier nicht DB-Spezifisches gebraucht wird (Mongo-OID)
	id, err := environment.Env.CommentModel.Create(comment, userID)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusCreated, Created{id})
}

// ListCommentsPubic returns all comments and their answers (limited)
func ListCommentsPublic(c *gin.Context) {

	comments, err := environment.Env.CommentModel.ListComments(c.Param("id"), "")
	if err != nil {
		// nothing found (not an error to the client)
		if err == apperror.ErrNoData {
			c.Status(http.StatusNoContent)
			return
		}
		// technical errors
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusOK, comments)
}
