package controllers

import (
	"fmt"
	"forza-garage/authentication"
	"forza-garage/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetUser sends a profile
func GetUser(c *gin.Context) {

	var apiError ErrorResponse

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
		switch err {
		case models.ErrInvalidFriend:
			apiError.Code = InvalidFriend
			apiError.Message = apiError.String(apiError.Code)
			c.JSON(http.StatusUnprocessableEntity, apiError)
		default:
			apiError.Code = SystemError
			apiError.Message = apiError.String(apiError.Code)
			fmt.Println(err)
			c.JSON(http.StatusInternalServerError, apiError)
		}
		return
	}

	// ToDO: Return updated usr struct?
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
		switch err {
		case models.ErrInvalidFriend:
			apiError.Code = InvalidFriend
			apiError.Message = apiError.String(apiError.Code)
			c.JSON(http.StatusUnprocessableEntity, apiError)
		default:
			apiError.Code = SystemError
			apiError.Message = apiError.String(apiError.Code)
			// fmt.Println(err)
			c.JSON(http.StatusInternalServerError, apiError)
		}
		return
	}

	// ToDO: Return updated usr struct?
}
