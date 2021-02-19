package controllers

import (
	"fmt"
	"forza-garage/authentication"
	"forza-garage/helpers"
	"forza-garage/models"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// UserExists maybe used to validate new accounts while typing into the form
func UserExists(c *gin.Context) {

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
		LoginName string `json:"loginName" binding:"required"`
	}{}

	// use 'shouldBind' so we can send customized messages
	if err := c.ShouldBindJSON(&data); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	exists := env.userModel.UserExists(data.LoginName)

	// wrap response into an object
	res := struct {
		Exists bool `json:"exists"`
	}{exists}

	c.JSON(http.StatusOK, res)
}

// EMailExists maybe used to validate new accounts while typing into the form
func EMailExists(c *gin.Context) {

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
		EMailAddress string `json:"eMailAddress" binding:"required"`
	}{}

	// use 'shouldBind' so we can send customized messages
	if err := c.ShouldBindJSON(&data); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	exists := env.userModel.EMailAddressExists(data.EMailAddress)

	// wrap response into an object
	res := struct {
		Exists bool `json:"exists"`
	}{exists}

	c.JSON(http.StatusOK, res)
}

// Register a new User
func Register(c *gin.Context) {

	var (
		err      error
		data     models.User
		apiError ErrorResponse
	)

	// short syntax (err "zentral" deklariert)
	if err = c.ShouldBindJSON(&data); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// Validated by the model
	// Die Prüfung der <User> Struktur erfolgt hier nur via ShouldBindJSON
	// da nicht alle Felder zentral erzwungen werden können (z. B. Password)
	// somit werden nur die für den Request benötigten Felder geprüft
	// auf diese Weise kann der Client übermitteln, was er will, je nach Design
	data.LoginName = strings.TrimSpace(data.LoginName)
	data.Password = strings.TrimSpace(data.Password)
	data.EMailAddress = strings.TrimSpace(data.EMailAddress) // ToDo: perhaps check for valid form

	// basically look for missing fields
	// len(data.LoginName) < 3|len(data.Password < 8|len(data.EMailAddress == 0)
	if len(data.LoginName) < 3 || len(data.Password) < 8 || len(data.EMailAddress) == 0 {
		apiError.Code = InvalidRequest
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// this also validates the user name, pwd etc.
	ID, err := env.userModel.CreateUser(data)
	if err != nil {
		fmt.Println(err)
		// ToDo: maybe check for an existing XBox-Tag and ask do u really want ... :-)
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	c.JSON(http.StatusOK, Created{ID})
}

// Login a user
func Login(c *gin.Context) {

	var (
		err       error
		givenUser models.User
		dbUser    *models.User
		apiError  ErrorResponse
	)

	// use std struct
	if err = c.ShouldBindJSON(&givenUser); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// check for required fields
	givenUser.LoginName = strings.TrimSpace(givenUser.LoginName)
	givenUser.Password = strings.TrimSpace(givenUser.Password)
	if len(givenUser.LoginName) == 0 || len(givenUser.Password) == 0 {
		apiError.Code = InvalidRequest
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnauthorized, apiError)
		return
	}

	// Benutzer in der DB suchen und das Profil laden
	dbUser, err = env.userModel.GetUserByName(givenUser.LoginName)
	if err != nil {
		// user does not exist
		if err == models.ErrInvalidUser {
			// send custom error message
			apiError.Code = InvalidLogin
			apiError.Message = apiError.String(apiError.Code)
			c.JSON(http.StatusUnauthorized, apiError)
			return
		}
		// "real" error
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	// übergibt das unverschlüsselte PWD vom Login und das verschlüsselte aus der DB
	granted := env.userModel.CheckPassword(givenUser.Password, *dbUser)
	if !granted {
		// send custom error message
		apiError.Code = InvalidLogin
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnauthorized, apiError)
		return
	}

	// create, register & save pair of AT/RT
	err = authentication.CreateTokens(c, dbUser.ID.Hex())
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	env.userModel.SetLastSeen(dbUser.ID)

	// passwort nicht erneut zurücksenden
	dbUser.Password = ""

	c.JSON(http.StatusOK, &dbUser)
}

// Logout löscht das Access Token in der Registry - ToDO: Immer ok liefern
// (kein DB-Zugriff nötig)
func Logout(c *gin.Context) {

	// Falls der Benutzer nicht mehr zurück kommen können soll,
	// kann auch das RT und das Cookie gelöscht werden

	// Damit im Client der CurrentUser (LocalStorage) und das Cookie gelöscht
	// werden können, soll das API keinen Fehler liefern

	au, _ := authentication.ExtractTokenMetadata(authentication.AT, c.Request)

	// nur at löschen, rt bleibt bestehen
	// in case of error the token might be expired
	_, _ = authentication.DeleteAuth(au.TokenUUID)

	// "Hard log-out" => AT, RT & Cookie löschen => auf allen Geräten ausloggen
	// auch für Testzwecke nützlich

	au, _ = authentication.ExtractTokenMetadata(authentication.RT, c.Request)

	// rt löschen
	// in case of error the token might be expired
	_, _ = authentication.DeleteAuth(au.TokenUUID)

	// Cookie löschen
	_ = helpers.DelCookie(c, os.Getenv("JWTCK_NAME"))

	c.Status(http.StatusOK)
}

// Refresh erzeugt ein neues AT wenn noch ein RT vorhanden ist - ToDo: evtl. ATs beschränken (wiederholte gültige Refreshes)
func Refresh(c *gin.Context) {

	var apiError ErrorResponse

	au, err := authentication.ExtractTokenMetadata(authentication.RT, c.Request)
	if err != nil {
		_, apiError = HandleError(err)
		c.JSON(http.StatusUnauthorized, apiError)
		return
	}

	// ist das RT noch gültig? (macht beim AT die Middleware)
	err = authentication.TokenValid(authentication.RT, c.Request)
	if err != nil {
		_, apiError = HandleError(err)
		c.JSON(http.StatusUnauthorized, apiError)
		return
	}

	// userID für die Ausstellung eines neues Token Pair
	userID, err := authentication.FetchAuth(au)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	// Die /refresh Route könnte auch eine leere Antwort zurückgeben. Vielleicht ist die erneute/aktualisierte
	// Lieferung von <User> mal sinnvoll, bspw. für aktualisierte Sicherheitsinformationen oder einen Zähler etc.
	dbUser, err := env.userModel.GetUserByID(userID)
	if err != nil {
		// user does not exist - erneute Prüfung eigentlich kaum nötig, kann aber noch mehr Sicherheit geben :-)
		if err == models.ErrInvalidUser {
			status, apiError := HandleError(err)
			c.JSON(status, apiError)
			return
		}
		// "real" error
		c.Status(http.StatusInternalServerError) // make client say "please try again later"
		return
	}

	// falls zu viele RTs (Clients) für den User in Umlauf sind, alle löschen, sonst nur das aktuelle
	// die ATs werden stehen gelassen; diese Clients können also noch damit arbeiten
	// ein neuer Refresh wird dann aber nicht mehr gehen
	deleted, err := authentication.DeleteAuths(authentication.RT, userID, au.TokenUUID)
	if err != nil || deleted == 0 { //if anything goes wrong
		apiError.Code = InvalidRequest
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// create, register & save pair of AT/RT
	err = authentication.CreateTokens(c, userID)
	if err != nil {
		_, apiError = HandleError(err)
		c.JSON(http.StatusUnauthorized, apiError)
		return
	}

	env.userModel.SetLastSeen(dbUser.ID)

	// passwort nicht erneut zurücksenden
	dbUser.Password = ""

	c.JSON(http.StatusOK, &dbUser)
}

// VerifyPassword is used whenever a password must be re-typed during a session
// (eg. changePassword or any actions that required increased security)
func VerifyPassword(c *gin.Context) {

	var (
		err       error
		givenUser models.User
		dbUser    *models.User
		apiError  ErrorResponse
	)

	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// use std struct (reduced fieldset)
	if err = c.ShouldBindJSON(&givenUser); err != nil {
		apiError.Code = InvalidJSON
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnprocessableEntity, apiError)
		return
	}

	// check for required fields
	givenUser.LoginName = strings.TrimSpace(givenUser.LoginName)
	givenUser.Password = strings.TrimSpace(givenUser.Password)
	if len(givenUser.LoginName) == 0 || len(givenUser.Password) == 0 {
		apiError.Code = InvalidRequest
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnauthorized, apiError)
		return
	}

	// wrap response into an object
	res := struct {
		Granted bool `json:"granted"`
	}{false}

	// Benutzer in der DB suchen und das Profil laden (via ID aus Token)
	dbUser, err = env.userModel.GetUserByID(userID)
	if err != nil {
		// user does not exist (return false)
		if err == models.ErrInvalidUser {
			c.JSON(http.StatusOK, res)
			return
		}
		// technical error
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	// Passt der Benutzername zur ID im Token?
	if givenUser.LoginName != dbUser.LoginName {
		c.JSON(http.StatusOK, res) // false (default)
		return
	}

	// übergibt das unverschlüsselte PWD vom Login und das verschlüsselte aus der DB
	res.Granted = env.userModel.CheckPassword(givenUser.Password, *dbUser)

	c.JSON(http.StatusOK, res)
}

// ChangePassword sets a new password
func ChangePassword(c *gin.Context) {

	var dbUser *models.User
	var apiError ErrorResponse

	// default auth-check
	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	// request data
	data := struct {
		LoginName   string `json:"loginName" binding:"required"`
		CurrentPWD  string `json:"currentPWD" binding:"required"` // extra-security if that's sent/checked again?
		NewPassword string `json:"newPWD" binding:"required"`
	}{}

	// let the Gin framework validate the request
	if err := c.BindJSON(&data); err != nil {
		return // wirft 400 - bad request
	}

	// simple cleansing
	data.LoginName = strings.TrimSpace(data.LoginName)
	data.CurrentPWD = strings.TrimSpace(data.CurrentPWD)
	data.NewPassword = strings.TrimSpace(data.NewPassword)

	// look for empty fields (Gin does not trim)
	if len(data.LoginName) == 0 || len(data.CurrentPWD) == 0 || len(data.NewPassword) < 8 {
		apiError.Code = InvalidRequest
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnauthorized, apiError)
		return
	}

	// re-load user's profile to perform additional security checks
	dbUser, err = env.userModel.GetUserByID(userID)
	if err != nil {
		// user does not exist
		if err == models.ErrInvalidUser {
			// report auth error
			apiError.Code = InvalidRequest
			apiError.Message = apiError.String(apiError.Code)
			c.JSON(http.StatusUnauthorized, apiError)
			return
		}
		// "real" error
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}

	// as an extra-security measure, compare given user name with the one referenced in the cookie
	if data.LoginName != dbUser.LoginName {
		apiError.Code = InvalidRequest
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnauthorized, apiError)
		return
	}

	// check the current password (again)
	granted := env.userModel.CheckPassword(data.CurrentPWD, *dbUser)
	if !granted {
		apiError.Code = InvalidRequest
		apiError.Message = apiError.String(apiError.Code)
		c.JSON(http.StatusUnauthorized, apiError)
		return
	}

	// ToDo: Validate new PWD (or include that in SetPWD)
	err = env.userModel.SetPassword(dbUser.ID, data.NewPassword)
	if err != nil {
		status, apiError := HandleError(err)
		c.JSON(status, apiError)
		return
	}
}
