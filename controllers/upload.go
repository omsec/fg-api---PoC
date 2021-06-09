package controllers

import (
	"fmt"
	"forza-garage/authentication"
	"forza-garage/environment"
	"forza-garage/helpers"
	"forza-garage/models"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/twinj/uuid"
)

// Upload is the generic file uploader for profiles
func UploadFile(c *gin.Context) {

	var (
		err        error
		apiError   ErrorResponse
		uploadInfo *models.UploadInfo
	)

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// (no post body available at forms)
	profileID := c.PostForm("profileId")
	profileType := c.PostForm("profileType")
	if profileID == "" || profileType == "" {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// single file
	file, err := c.FormFile("file")
	if err != nil {
		fmt.Println(err)
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// generate file name & initialize metadata
	uploadInfo = new(models.UploadInfo)
	uploadInfo.UploadedID = helpers.ObjectID(userID) // executive user from token
	uploadInfo.SysFileName = profileType + "_" + uuid.NewV4().String() + filepath.Ext(file.Filename)
	uploadInfo.OrigFileName = file.Filename
	uploadInfo.Description = c.PostForm("description")
	uploadInfo.URL = os.Getenv("API_HOME") + ":" + os.Getenv("API_PORT") + environment.UploadEndpoint + "/" + uploadInfo.SysFileName

	// https://www.devdungeon.com/content/working-files-go

	stage := os.Getenv("UPLOAD_STAGE") + "/" + uploadInfo.SysFileName

	// Upload the file to specific stage
	err = c.SaveUploadedFile(file, stage)
	if err != nil {
		fmt.Println(err)
		apiError.Code = SystemError
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusInternalServerError, apiError)
		return
	}

	// move file to destination
	dst := os.Getenv("UPLOAD_TARGET") + "/" + uploadInfo.SysFileName
	err = os.Rename(stage, dst)
	if err != nil {
		fmt.Println(err)
		apiError.Code = SystemError
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusInternalServerError, apiError)
		return
	}

	// update meta data (registry)
	err = environment.Env.UploadModel.SaveMetaData(profileID, profileType, uploadInfo)
	if err != nil {
		fmt.Println(err)
		apiError.Code = SystemError
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusInternalServerError, apiError)
		return
	}

	c.JSON(http.StatusCreated, Uploaded{
		uploadInfo.URL,
		uploadInfo.StatusCode,
		uploadInfo.StatusText,
	})

}
