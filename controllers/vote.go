package controllers

import (
	"fmt"
	"forza-garage/authentication"
	"forza-garage/environment"
	"forza-garage/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Vote(c *gin.Context) {

	var (
		err      error
		data     models.Vote
		apiError ErrorResponse
	)

	// for enhanced security, read user from token
	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// use "shouldBind" not all fields are required in this context
	if err = c.Bind(&data); err != nil {
		fmt.Println("ress")
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	/*
		// validate request
		course, err := environment.Env.CourseModel.Validate(data)
		if err != nil {
			status, apiError := HandleError(err)
			c.JSON(status, apiError)
			return
		}
	*/

	err = environment.Env.VoteModel.Vote(data.ProfileID, userID, data.Vote)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

}
