package controllers

import (
	"forza-garage/authentication"
	"forza-garage/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Test is what it seems :-)
func Test(c *gin.Context) {

	var apiError ErrorResponse

	/*
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	*/

	/*c.Status(http.StatusInternalServerError)
	return*/

	// userID (currentUser) could be used to check a user's permission to view another profile
	/*
		userID, err := authentication.Authenticate(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, err.Error())
			return
		}
	*/

	// fehlender parameter muss nicht geprüft werden, sonst wär's eine andere route
	user, err := env.userModel.GetUserByID("5feb2473b4d37f7f0285847a")
	if err != nil {
		apiError.Code = InvalidRequest
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// don't send password hash
	user.Password = ""

	c.JSON(http.StatusOK, &user)
}

// GetUser sends a profile
func GetUser(c *gin.Context) {

	var apiError ErrorResponse

	/*
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return*
	*/
	/*
		c.Status(http.StatusInternalServerError)
		return
	*/

	// userID (currentUser) could be used to check a user's permission to view another profile
	/*
		userID, err := authentication.Authenticate(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, err.Error())
			return
		}
	*/

	// fehlender parameter muss nicht geprüft werden, sonst wär's eine andere route
	user, err := env.userModel.GetUserByID(c.Param("id"))
	if err != nil {
		apiError.Code = InvalidRequest
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// don't send password hash
	user.Password = ""

	c.JSON(http.StatusOK, &user)
}

// GetFriends sends a profile
func GetFriends(c *gin.Context) {

	// userID (currentUser) could be used to check a user's permission to view another profile
	/*
		userID, err := authentication.Authenticate(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, err.Error())
			return
		}
	*/

	// fehlender parameter muss nicht geprüft werden, sonst wär's eine andere route
	friends, err := env.userModel.GetFriends(c.Param("id"))
	if err != nil {
		// nothing found (not an error to the client)
		if err == models.ErrNoData {
			c.Status(http.StatusNoContent)
			return
		}
		// technical errors
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusOK, &friends)
}

// GetFollowings lists all people someone's following
func GetFollowings(c *gin.Context) {

	// userID (currentUser) could be used to check a user's permission to view another profile
	/*
		userID, err := authentication.Authenticate(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, err.Error())
			return
		}
	*/

	// fehlender parameter muss nicht geprüft werden, sonst wär's eine andere route
	friends, err := env.userModel.GetFollowings(c.Param("id"))
	if err != nil {
		// nothing found (not an error to the client)
		if err == models.ErrNoData {
			c.Status(http.StatusNoContent)
			return
		}
		// technical errors
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusOK, &friends)
}

// GetFollowers lists all people who are following someone
func GetFollowers(c *gin.Context) {

	// userID (currentUser) could be used to check a user's permission to view another profile
	/*
		userID, err := authentication.Authenticate(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, err.Error())
			return
		}
	*/

	// fehlender parameter muss nicht geprüft werden, sonst wär's eine andere route
	followers, err := env.userModel.GetFollowers(c.Param("id"))
	if err != nil {
		// nothing found (not an error to the client)
		if err == models.ErrNoData {
			c.Status(http.StatusNoContent)
			return
		}
		// technical errors
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusOK, &followers)
}

// AddFriend adds someone to the user's friendlist
func AddFriend(c *gin.Context) {

	var apiError ErrorResponse

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// anonymous struct used to receive input (POST BODY)
	// ToDo: mehrere auf einmal vorsehen - nötig?
	data := struct {
		FriendID string `json:"friendID" binding:"required"`
	}{}

	// use 'shouldBind' so we can send customized messages
	if err := c.ShouldBindJSON(&data); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	err = env.userModel.AddFriend(userID, data.FriendID)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}
}

// RemoveFriend adds someone to the user's friendlist
func RemoveFriend(c *gin.Context) {

	var apiError ErrorResponse

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// anonymous struct used to receive input (POST BODY)
	// ToDo: mehrere auf einmal vorsehen - nötig?
	data := struct {
		FriendID string `json:"friendID" binding:"required"`
	}{}

	// use 'shouldBind' so we can send customized messages
	if err := c.ShouldBindJSON(&data); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	err = env.userModel.RemoveFriend(userID, data.FriendID)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}
}

// FollowUser adds someone to the user's friendlist
func FollowUser(c *gin.Context) {

	var apiError ErrorResponse

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// anonymous struct used to receive input (POST BODY)
	// ToDo: mehrere auf einmal vorsehen - nötig?
	data := struct {
		UserID string `json:"userID" binding:"required"` // user to be followed
	}{}

	// use 'shouldBind' so we can send customized messages
	if err := c.ShouldBindJSON(&data); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	err = env.userModel.FollowUser(userID, data.UserID)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}
}
