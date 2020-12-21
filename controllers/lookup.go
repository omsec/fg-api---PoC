package controllers

import (
	"fmt"
	"forza-garage/database"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ListLookups bla
func ListLookups(c *gin.Context) {
	lookups, err := database.GetLookups()
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusNoContent, nil)
		return
	}

	c.JSON(http.StatusOK, lookups)
}
