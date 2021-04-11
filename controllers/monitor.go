package controllers

import (
	"forza-garage/authentication"
	"forza-garage/environment"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CountRequests(c *gin.Context) {

	_, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	c.JSON(http.StatusOK, environment.Env.Requests.Count())
}

func DumpRequests(c *gin.Context) {

	_, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	c.JSON(http.StatusOK, environment.Env.Requests.Dump(50))
}

func FlushRequests(c *gin.Context) {

	_, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// ToDO: Add error for this purpose ;-)
	environment.Env.Requests.Flush()

	c.Status(http.StatusOK)
}
