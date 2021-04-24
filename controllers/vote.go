package controllers

import (
	"forza-garage/authentication"
	"forza-garage/environment"
	"forza-garage/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CastVote registers a new vote or removes a revoked one. It also calcalutes the new rating and lower boundary to sort the profiles
// ToDO: make generic - objType passed in POST body
func CastVoteCourse(c *gin.Context) {

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

	profileVotes, err := environment.Env.VoteModel.CastVote(data.ProfileID, data.ProfileType, userID, data.Vote, environment.Env.CourseModel.SetRating)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusOK, profileVotes)
}

// GetVotesPublic returns the current votes for and against a profile
// http://localhost:3000/courses/public/6060491beab278c482d04ed8/votes
func GetVotesPublic(c *gin.Context) {

	var (
		profileId = c.Param("id")
	)

	profileVotes, err := environment.Env.VoteModel.GetVotes(profileId, "")
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusOK, profileVotes)
}

// GetVotesMember returns the current votes for and against a profile as well as a user's action
// http://localhost:3000/courses/member/6060491beab278c482d04ed8/votes
func GetVotesMember(c *gin.Context) {

	var profileId = c.Param("id")

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	profileVotes, err := environment.Env.VoteModel.GetVotes(profileId, userID)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusOK, profileVotes)
}
