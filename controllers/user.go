package controllers

import (
	"forza-garage/authentication"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetUser sends a profile
func GetUser(c *gin.Context) {

	// userID (currentUser) could be used to check a user's permission to view another profile
	_, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, err.Error())
		return
	}

	// fehlender parameter muss nicht geprüft werden, sonst wär's eine andere route
	user, err := env.userModel.GetUserByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNoContent, nil)
		return
	}

	// don't send password hash
	user.Password = ""

	c.JSON(http.StatusOK, &user)
	//c.JSON(http.StatusInternalServerError, "error")
}
