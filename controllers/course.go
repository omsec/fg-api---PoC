package controllers

import (
	"forza-garage/apperror"
	"forza-garage/authentication"
	"forza-garage/environment"
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
	course, err := environment.Env.CourseModel.Validate(data)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	// userID als Parameter, damit hier nicht DB-Spezifisches gebraucht wird (Mongo-OID)
	id, err := environment.Env.CourseModel.CreateCourse(course, userID)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusCreated, Created{id})
}

// ListCoursesPublic returns a list of racing tracks
// format => http://localhost:3000/courses/public?searchMode=2&game=0&series=0&series=2&search=test
func ListCoursesPublic(c *gin.Context) {

	var apiError ErrorResponse

	// Error maybe ignored here
	// no user available/needed for the public service.
	// the model will assign the default profile/role to it, without the need of a DB access
	userID := ""

	//var search *models.CourseSearchParams
	search := new(models.CourseSearchParams)

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

	// variable wiederholt sich einfach im url
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

	search.SearchTerm = c.Query("search")

	// ToDo: Lang
	// use language submitted by client for anonymous users (rather than the one stored in database)
	/*
		if userID == "" {
			i, _ := strconv.Atoi(c.Request.Header.Get("Language")) // default 0, EN
			search.Credentials.LanguageCode = int32(i)
		}
	*/

	// nötig?
	// searchTerm = strings.TrimSpace(data.SearchTerm)
	// fmt.Println(data.SearchTerm)

	courses, err := environment.Env.CourseModel.SearchCourses(search, userID)
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

	c.JSON(http.StatusOK, courses)
}

// ListCoursesMember returns a list of racing tracks for logged-in users
// format => http://localhost:3000/courses/member?searchMode=2&game=0&series=0&series=2&search=test
func ListCoursesMember(c *gin.Context) {

	var apiError ErrorResponse

	/*apiError.Code = InvalidJSON
	apiError.Message = apiError.String(apiError.Code)
	c.JSON(http.StatusUnprocessableEntity, apiError)
	return

	/*c.Status(http.StatusInternalServerError)
	return*/

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	//var search *models.CourseSearchParams
	search := new(models.CourseSearchParams)

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

	// variable wiederholt sich einfach im url
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

	search.SearchTerm = c.Query("search")

	// ToDo: Language
	// use language submitted by client for anonymous users (rather than the one stored in database)
	/*
		if userID == "" {
			i, _ := strconv.Atoi(c.Request.Header.Get("Language")) // default 0, EN
			search.Credentials.LanguageCode = int32(i)
		}
	*/

	// nötig?
	// searchTerm = strings.TrimSpace(data.SearchTerm)
	// fmt.Println(data.SearchTerm)

	courses, err := environment.Env.CourseModel.SearchCourses(search, userID)
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

	c.JSON(http.StatusOK, courses)
}

// GetCoursePublic returns the specified track
func GetCoursePublic(c *gin.Context) {

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

	// no user available/required for the public service
	userID := ""

	// ToDO: use language submitted by client for anonymous users (rather than the one stored in database)
	/*
		if userID == "" {
			i, _ := strconv.Atoi(c.Request.Header.Get("Language")) // default 0, EN
			credentials.LanguageCode = int32(i)
		}*/

	// muss nicht auf null geprüft werden, denn ohne Parameter ist es eine andere Route (wie in Angular)
	// typ wird automatisch gesetzt (hier STR, könnte auch numerisch sein)
	var id = c.Param("id")

	data, err = environment.Env.CourseModel.GetCourse(id, userID)
	if err != nil {
		switch err {
		// record not found is not an error to the client here
		case apperror.ErrNoData:
			c.Status(http.StatusNoContent)
		default:
			status, apiError := HandleError(err)
			c.JSON(status, apiError)
		}
		return
	}

	c.JSON(http.StatusOK, data)
}

func GetCourseMember(c *gin.Context) {

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

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// ToDO: use language submitted by client for anonymous users (rather than the one stored in database)
	/*
		if userID == "" {
			i, _ := strconv.Atoi(c.Request.Header.Get("Language")) // default 0, EN
			credentials.LanguageCode = int32(i)
		}*/

	// muss nicht auf null geprüft werden, denn ohne Parameter ist es eine andere Route (wie in Angular)
	// typ wird automatisch gesetzt (hier STR, könnte auch numerisch sein)
	var id = c.Param("id")

	data, err = environment.Env.CourseModel.GetCourse(id, userID)
	if err != nil {
		switch err {
		// record not found is not an error to the client here
		case apperror.ErrNoData:
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

	// use "shouldBind" not all fields are required in this context (eg. MetaInfo.recVer is requred)
	if err = c.ShouldBindJSON(&data); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// validate request (inhaltlich)
	course, err := environment.Env.CourseModel.Validate(data)
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

	err = environment.Env.CourseModel.UpdateCourse(course, userID)
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

	exists, err := environment.Env.CourseModel.ForzaSharingExists(data.ForzaSharing)
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
