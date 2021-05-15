package controllers

import (
	"fmt"
	"forza-garage/authentication"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Upload is the generic file uploader for profiles
func Upload(c *gin.Context) {

	// userID
	_, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// call resp model func to save file meta data

	// https://github.com/gin-gonic/gin#single-file

	// single file
	file, err := c.FormFile("file")
	if err != nil {
		fmt.Println(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	dst := "test.jpg"

	// Upload the file to specific dst.
	c.SaveUploadedFile(file, dst)

	c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", file.Filename))
}
