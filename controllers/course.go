package controllers

import (
	"forza-garage/authentication"
	"forza-garage/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// AddCourse creates a new route
func AddCourse(c *gin.Context) {

	var (
		err      error
		data     models.Course
		apiError ErrorResponse
	)

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// use "shouldBind" not all fields are required in this context
	if err = c.ShouldBindJSON(&data); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// validate request
	course, err := env.courseModel.Validate(data)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	// ToDO: Evtl. vereinfachen mit Helpers
	course.MetaInfo.CreatedID = models.ObjectID(userID)
	course.MetaInfo.CreatedName, err = env.userModel.GetUserName(userID)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	id, err := env.courseModel.CreateCourse(course)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusOK, Created{id})
}

// ListCourses returns a list of racing tracks
// format => http://localhost:3000/courses?searchMode=2&game=0&series=0&series=2&search=test
func ListCourses(c *gin.Context) {

	var apiError ErrorResponse

	/*apiError.Code = InvalidJSON
	apiError.Message = apiError.String(apiError.Code)
	c.JSON(http.StatusUnprocessableEntity, apiError)
	return

	/*c.Status(http.StatusInternalServerError)
	return*/

	// Error maybe ignored here
	// Service is public, however members receive more results (and do need to wait for another request)
	userID, _ := authentication.Authenticate(c.Request)

	var search *models.CourseSearchParams
	search = new(models.CourseSearchParams)

	i, err := strconv.Atoi(c.Query("searchMode"))
	if err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}
	search.SearchMode = i

	i, err = strconv.Atoi(c.Query("game"))
	if err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}
	search.GameCode = int32(i)

	series := c.QueryArray("series")
	if series == nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}
	for _, str := range series {
		i, err = strconv.Atoi(str)
		// ignore invalid codes
		if err == nil {
			search.SeriesCodes = append(search.SeriesCodes, int32(i))
		}
	}
	if search.SeriesCodes == nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// variable wiederholt sich einfach im url
	search.SearchTerm = c.Query("search")
	// since models shouldn't open DB-connections on their own
	// the user credentials are passed to it
	// errors maybe ignored here and will be treated as anonymous user
	search.Credentials, _ = env.userModel.GetCredentials(userID)

	// use language submitted by client for anonymous users (rather than the one stored in database)
	if userID == "" {
		i, _ := strconv.Atoi(c.Request.Header.Get("Language")) // default 0, EN
		search.Credentials.LanguageCode = int32(i)
	}

	// nötig?
	// searchTerm = strings.TrimSpace(data.SearchTerm)
	// fmt.Println(data.SearchTerm)

	courses, err := env.courseModel.SearchCourses(search)
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

	c.JSON(http.StatusOK, courses)
}

// GetCourse returns the specified track
func GetCourse(c *gin.Context) {

	var (
		err  error
		data *models.Course
	)

	/*
		var apiError ErrorResponse
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	*/

	// no error checking because it's optional (public courses only)
	userID, _ := authentication.Authenticate(c.Request)
	credentials, _ := env.userModel.GetCredentials(userID)

	// use language submitted by client for anonymous users (rather than the one stored in database)
	if userID == "" {
		i, _ := strconv.Atoi(c.Request.Header.Get("Language")) // default 0, EN
		credentials.LanguageCode = int32(i)
	}

	// muss nicht auf null geprüft werden, denn ohne Parameter ist es eine andere Route (wie in Angular)
	// typ wird automatisch gesetzt (kann aber STR sein)
	var id = c.Param("id")

	data, err = env.courseModel.GetCourse(id, credentials)
	if err != nil {
		switch err {
		// record not found is not an error to the client here
		case models.ErrNoData:
			c.Status(http.StatusNoContent)
		default:
			status, apiError := HandleError(err)
			c.JSON(status, apiError)
		}
		return
	}

	c.JSON(http.StatusOK, data)
}

// UpdateCourse modifies "core" fields
func UpdateCourse(c *gin.Context) {

	var (
		err      error
		data     models.Course
		apiError ErrorResponse // declared here to raise own errors
	)

	// evtl. kurz syntaxc if ok.... mit getCred...
	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	credentials, _ := env.userModel.GetCredentials(userID)

	// ToDo: Evtl. eigene data struct machen, da RecVer vorhanden sein muss
	// fehlend != 0

	// use "shouldBind" not all fields are required in this context (eg. MetaInfo.recVer is requred)
	if err = c.ShouldBindJSON(&data); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// validate request (inhaltlich)
	course, err := env.courseModel.Validate(data)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}
	/*
		if err != nil {
			switch err {
			case models.ErrCourseNameMissing:
				apiError.Code = CourseNameMissing
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
	*/

	err = env.courseModel.UpdateCourse(course, credentials)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.Status(http.StatusNoContent) // evtl. auch 205

}

// Additional & Helper Services

// ExistsForzaShare checks if a given Forza Sharing Code is already in use
// (used for typing-checks in clients)
func ExistsForzaShare(c *gin.Context) {

	var apiError ErrorResponse

	/*
		// falls später die userID/Rolle geprüft werden soll
		userID, err := authentication.Authenticate(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, authentication.ErrUnauthorized.Error())
			return
		}
	*/

	// anonymous struct used to receive input (POST BODY)
	data := struct {
		ForzaSharing int32 `json:"ForzaSharing" binding:"required"`
	}{}

	// use 'shouldBind' so we can send customized messages
	if err := c.ShouldBindJSON(&data); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	exists, err := env.courseModel.ForzaSharingExists(data.ForzaSharing)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	// wrap response into an object
	res := struct {
		Exists bool `json:"exists"`
	}{exists}

	c.JSON(http.StatusOK, res)
}
