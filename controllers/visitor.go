package controllers

import (
	"fmt"
	"forza-garage/apperror"
	"forza-garage/authentication"
	"forza-garage/environment"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// http://localhost:3000/stats/visits?id=604b6859f09f3aeecc9215c5&startDT=2021-03-20
func GetVisits(c *gin.Context) {

	var (
		err error
		//data     models.Course
		apiError ErrorResponse
	)

	id := c.Query("id")
	if id == "" {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	var startDT time.Time // = time.Now()

	startStr := c.Query("startDT")
	if startStr == "" {
		// default: 7 days back (starting at 00:00:00)
		// https://www.golangprograms.com/subtract-n-number-of-year-month-day-hour-minute-second-millisecond-microsecond-and-nanosecond-to-current-date-time.html
		startDT = time.Now().AddDate(0, 0, -7)
		// https://stackoverflow.com/questions/36988681/time-time-round-to-day
		//startDT = time.Date(startDT.Year(), startDT.Month(), startDT.Day(), 0, 0, 0, 0, startDT.Location())
		startDT = time.Date(startDT.Year(), startDT.Month(), startDT.Day(), 0, 0, 0, 0, startDT.UTC().Location())
	} else {
		// https://forum.golangbridge.org/t/convert-string-to-date-in-yyyy-mm-dd-format/6026/2
		startDT, err = time.Parse("2006-01-02", startStr) // seems magic date
		if err != nil {
			fmt.Println(err)
			apiError.Code = InvalidRequest
			apiError.Message = apiError.String(apiError.Code)
			c.JSON(http.StatusUnprocessableEntity, apiError)
			return
		}
	}

	visits, err := environment.Env.Tracker.GetVisits("course", id, startDT)
	if err != nil {
		fmt.Println(err)
		apiError.Code = InvalidRequest // ToDO: evtl. intServ oder genauer
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// wrap response into an object
	res := struct {
		Visits int64 `json:"visits"`
	}{visits}

	c.JSON(http.StatusOK, res)

}

// http://localhost:3000/stats/visits?id=604b6859f09f3aeecc9215c5&startDT=2021-03-20
func ListVisitors(c *gin.Context) {

	var (
		err error
		//data     models.Course
		apiError ErrorResponse
	)

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	id := c.Query("id")
	if id == "" {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	var startDT time.Time // = time.Now()

	startStr := c.Query("startDT")
	if startStr == "" {
		// default: 7 days back (starting at 00:00:00)
		// https://www.golangprograms.com/subtract-n-number-of-year-month-day-hour-minute-second-millisecond-microsecond-and-nanosecond-to-current-date-time.html
		startDT = time.Now().AddDate(0, 0, -7)
		// https://stackoverflow.com/questions/36988681/time-time-round-to-day
		//startDT = time.Date(startDT.Year(), startDT.Month(), startDT.Day(), 0, 0, 0, 0, startDT.Location())
		startDT = time.Date(startDT.Year(), startDT.Month(), startDT.Day(), 0, 0, 0, 0, startDT.UTC().Location())
	} else {
		fmt.Println(startStr)
		// https://forum.golangbridge.org/t/convert-string-to-date-in-yyyy-mm-dd-format/6026/2
		startDT, err = time.Parse("2006-01-02", startStr) // seems magic date
		if err != nil {
			fmt.Println(err)
			apiError.Code = InvalidRequest
			apiError.Message = apiError.String(apiError.Code)
			c.JSON(http.StatusUnprocessableEntity, apiError)
			return
		}
	}

	visitors, err := environment.Env.Tracker.ListVisitors(id, startDT, userID)
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

	c.JSON(http.StatusOK, visitors)

}
