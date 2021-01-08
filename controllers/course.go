package controllers

import (
	"fmt"
	"forza-garage/authentication"
	"forza-garage/models"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AddCourse creates a new route
func AddCourse(c *gin.Context) {

	var (
		err  error
		data models.Course
	)

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, authentication.ErrUnauthorized.Error())
		return
	}

	// use "shouldBind" not all fields are required in this context
	if err = c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusUnprocessableEntity, ErrInvalidRequest.Error())
		return
	}

	// validate request
	course, err := env.courseModel.Validate(data)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, err.Error())
		return
	}

	// ToDO: Evtl. vereinfachen mit Helpers
	course.MetaInfo.CreatedID = models.ObjectID(userID)
	course.MetaInfo.CreatedName, err = env.userModel.GetUserName(userID)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	id, err := env.courseModel.CreateCourse(course)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, err.Error())
		return
	}

	c.JSON(http.StatusOK, Created{id})
}

// ListCourses returns a list of racing tracks
func ListCourses(c *gin.Context) {

	// Error maybe ignored here
	// Service is public, however members receive more results (and do need to wait for another request)
	userID, _ := authentication.Authenticate(c.Request)

	fmt.Println(userID)

	data := struct {
		SearchTerm string `json:"searchTerm"`
	}{}

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusUnprocessableEntity, ErrInvalidRequest.Error())
		return
	}

	data.SearchTerm = strings.TrimSpace(data.SearchTerm)

	// any error would be considered "anonymous user"
	credentials, _ := env.userModel.GetCredentials(userID)

	fmt.Println(credentials)

	courses, err := env.courseModel.SearchCourses(data.SearchTerm)
	if err != nil {
		// nothing found
		if err == models.ErrNoData {
			c.Status(http.StatusNoContent)
			return
		}
		// technical errors
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, courses)

}
