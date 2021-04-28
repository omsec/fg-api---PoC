package controllers

import (
	"forza-garage/apperror"
	"forza-garage/authentication"
	"forza-garage/environment"
	"forza-garage/helpers"
	"forza-garage/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CastVote registers a new vote or removes a revoked one. It also calcalutes the new rating and lower boundary to sort the profiles
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

	// apply userID from token (username resolved in model)
	data.UserID = helpers.ObjectID(userID)

	// inject SetRating based on domain
	var profileVotes *models.ProfileVotes
	switch data.ProfileType {
	case "course":
		profileVotes, err = environment.Env.VoteModel.CastVote(data, environment.Env.CourseModel.SetRating)
	case "comment":
		profileVotes, err = environment.Env.VoteModel.CastVote(data, environment.Env.CommentModel.SetRating)
	default:
		apiError.Code = SystemError
		apiError.Message = apiError.String(apiError.Code)
		// fmt.Println(err)
		c.JSON(http.StatusInternalServerError, apiError)
	}
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusOK, profileVotes)
}

// GetUserVote returns the vote of a user to a profile - entfernt
// http://localhost:3000/user/vote?pId=6055d819671e62579fcc2151
/*
func GetUserVote(c *gin.Context) {

	var profileId = c.Query("pId")

	// always read userID from token (param is ignored)
	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	vote, err := environment.Env.VoteModel.GetUserVote(profileId, userID)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	// wrap response into an object
	res := struct {
		Vote int32 `json:"vote"`
	}{vote}

	c.JSON(http.StatusOK, res)
}
*/

// GetUserVotes returns the votes of a user to profiles of given type
// http://localhost:3000/users/601526e8a468e8973193facd/votes?pDomain=course
func GetUserVotes(c *gin.Context) {

	var domain = c.Query("pDomain")

	// always read userID from token (param is ignored)
	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	votes, err := environment.Env.VoteModel.GetUserVotes(domain, userID)
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

	c.JSON(http.StatusOK, votes)
}

// Nicht mehr benutzt
// GetVotesPublic returns the current votes for and against a profile
// http://localhost:3000/courses/public/6060491beab278c482d04ed8/votes
/*
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
*/
