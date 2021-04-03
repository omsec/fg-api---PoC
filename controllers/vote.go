package controllers

import (
	"fmt"
	"forza-garage/authentication"
	"forza-garage/environment"
	"forza-garage/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CastVote(c *gin.Context) {

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

	err = environment.Env.VoteModel.CastVote(data.ProfileID, userID, data.Vote)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

}

// GetVotes returns the current votes for and against a profile as well as a user's action
// ToDo: eine public/member URL (gleiche model funktion) wäre sicherer vor manipulation (user aus token lesen)
// http://localhost:3000/courses/public/6060491beab278c482d04ed8/votes?userId=5feb2473b4d37f7f0285847a
func GetVotes(c *gin.Context) {

	var (
		profileId = c.Param("id")
		userId    = c.Query("userId")
	)

	profileVotes, err := environment.Env.VoteModel.GetVotes(profileId, userId)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusOK, profileVotes)
}
