package controllers

import (
	"forza-garage/authentication"
	"forza-garage/models"
	"net/http"

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

	// ToDO: Evtl. vereinfachen mit Helpers
	course.MetaInfo.CreatedID = models.ObjectID(userID)
	course.MetaInfo.CreatedName, err = env.userModel.GetUserName(userID)
	if err != nil {
		switch err {
		case models.ErrInvalidUser:
			apiError.Code = InvalidRequest
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

	id, err := env.courseModel.CreateCourse(course)
	if err != nil {
		switch err {
		case models.ErrForzaSharingCodeTaken:
			apiError.Code = ForzaShareTaken
			apiError.Message = apiError.String(apiError.Code)
			c.JSON(http.StatusUnprocessableEntity, apiError)
		default:
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, Created{id})
}

// ListCourses returns a list of racing tracks
// format => http://localhost:3000/courses?game=fh4&search=roger
func ListCourses(c *gin.Context) {

	var apiError ErrorResponse

	// Error maybe ignored here
	// Service is public, however members receive more results (and do need to wait for another request)
	userID, _ := authentication.Authenticate(c.Request)

	var search *models.CourseSearch
	search = new(models.CourseSearch)

	search.UserID = userID
	search.GameText = c.Query("game")
	search.SearchTerm = c.Query("search")

	// since models shouldn't open DB-connections on their own
	// the user credentials are passed to it

	// TODO:
	// Funktion umbauen, dass ohne UserID die Def-Credentials kommen (role = Guest)
	// Language soll als Custom Header übergeben werden (bei anonymen nicht vorhanden in DB)
	// in search: Statt cred == nil halt cred.ADMIN (alles andere wäre dann GUEST/ANON (:nur ALL) oder Member (:Friends/Own))
	if userID != "" {
		// errors maybe ignored here, nil credentials will be treated as anonymous user
		search.Credentials, _ = env.userModel.GetCredentials(userID)
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
		apiError.Code = SystemError
		apiError.Message = apiError.String(apiError.Code)
		// fmt.Println(err)
		c.JSON(http.StatusInternalServerError, apiError)
		return
	}

	c.JSON(http.StatusOK, courses)
}

// GetCourse returns the specified track
func GetCourse(c *gin.Context) {

	var apiError ErrorResponse

	var (
		err  error
		data *models.Course
	)

	// ToDo: pass userID to model
	// no error checking because it's optional (public courses only)
	// userID, _ = authentication.Authenticate(c.Request)

	// muss nicht auf null geprüft werden, denn ohne Parameter ist es eine andere Route (wie in Angular)
	// typ wird automatisch gesetzt (kann aber STR sein)
	var id = c.Param("id")

	data, err = env.courseModel.GetCourse(id)
	if err != nil {
		// ToDO: Check Vsibility
		switch err {
		case models.ErrNoData:
			c.Status(http.StatusNoContent)
		default:
			apiError.Code = SystemError
			apiError.Message = apiError.String(apiError.Code)
			// fmt.Println(err)
			c.JSON(http.StatusInternalServerError, apiError)
		}
		return
	}

	c.JSON(http.StatusOK, data)
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
		apiError.Code = SystemError
		apiError.Message = apiError.String(apiError.Code)
		// fmt.Println(err)
		c.JSON(http.StatusInternalServerError, apiError)
		return
	}

	// wrap response into an object
	res := struct {
		Exists bool `json:"exists"`
	}{exists}

	c.JSON(http.StatusOK, res)
}
