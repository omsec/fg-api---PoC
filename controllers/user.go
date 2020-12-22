package controllers

import (
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

	data := struct {
		LoginName string `json:"loginName" binding:"required"`
	}{}

	// short syntax (err "zentral" deklariert)
	if err := c.BindJSON(&data); err != nil {
		return
	}

	// ToDo: More validation (trim ' ', len..?)
	// Basic Validation/Cleansing in helper Proc?
	data.LoginName = strings.TrimSpace(data.LoginName)
	if len(data.LoginName) < 3 {
		c.Status(http.StatusUnprocessableEntity)
		return
	}

	b := env.userModel.UserExists(data.LoginName)
	c.JSON(http.StatusOK, b)
}

// Register a new User
func Register(c *gin.Context) {

	var (
		err  error
		data models.User
	)

	// short syntax (err "zentral" deklariert)
	if err = c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusUnprocessableEntity, "invalid json")
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
		c.JSON(http.StatusUnprocessableEntity, "invalid data")
		return
	}

	// this also validates the user name, pwd etc.
	ID, err := env.userModel.CreateUser(data)
	if err != nil {
		// ToDo: maybe check for an existing XBox-Tag :-)
		switch err {
		case models.ErrUserNameNotAvailable:
			c.JSON(http.StatusUnprocessableEntity, err.Error())
		case models.ErrEMailAddressTaken:
			c.JSON(http.StatusUnprocessableEntity, err.Error())
		default:
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, ID) //std-struktur? ID: 0 - wo zentral definieren? package model.response ?
}

// Login a user
func Login(c *gin.Context) {

	var (
		err       error
		givenUser models.User
		dbUser    *models.User
	)

	// use std struct
	if err = c.ShouldBindJSON(&givenUser); err != nil {
		c.JSON(http.StatusUnprocessableEntity, "invalid json")
		return
	}

	// check for required fields
	givenUser.LoginName = strings.TrimSpace(givenUser.LoginName)
	givenUser.Password = strings.TrimSpace(givenUser.Password)
	if len(givenUser.LoginName) == 0 || len(givenUser.Password) == 0 {
		c.JSON(http.StatusForbidden, models.ErrInvalidUser.Error())
		return
	}

	// Benutzer in der DB suchen und das Profil laden
	dbUser, err = env.userModel.GetUserByName(givenUser.LoginName)
	if err != nil {
		// user does not exist
		if err == models.ErrInvalidUser {
			c.JSON(http.StatusUnauthorized, err.Error())
			return
		}
		// "real" error
		c.JSON(http.StatusInternalServerError, nil) // make client say "please try again later"
		return
	}

	// übergibt das unverschlüsselte PWD vom Login und das verschlüsselte aus der DB
	granted := env.userModel.CheckPassword(givenUser.Password, *dbUser)
	if !granted {
		c.JSON(http.StatusForbidden, models.ErrInvalidUser.Error())
		return
	}

	// create, register & save pair of AT/RT
	err = authentication.CreateTokens(c, dbUser.ID.Hex())
	if err != nil {
		c.JSON(http.StatusUnauthorized, err.Error())
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

	au, err := authentication.ExtractTokenMetadata(authentication.AT, c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, authentication.ErrUnauthorized.Error())
		return
	}

	// nur at löschen, rt bleibt bestehen
	deleted, delErr := authentication.DeleteAuth(au.TokenUUID)
	if delErr != nil || deleted == 0 {
		c.JSON(http.StatusUnauthorized, authentication.ErrUnauthorized.Error())
		return
	}

	// "Hard log-out" => AT, RT & Cookie löschen => auf allen Geräten ausloggen
	// auch für Testzwecke nützlich

	au, err = authentication.ExtractTokenMetadata(authentication.RT, c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, authentication.ErrUnauthorized.Error())
		return
	}

	// rt löschen
	deleted, delErr = authentication.DeleteAuth(au.TokenUUID)
	if delErr != nil || deleted == 0 {
		c.JSON(http.StatusUnauthorized, authentication.ErrUnauthorized.Error())
		return
	}

	// Cookie löschen
	_ = helpers.DelCookie(c, os.Getenv("JWTCK_NAME"))
}

// Refresh erzeugt ein neues AT wenn noch ein RT vorhanden ist - ToDo: evtl. ATs beschränken (wiederholte gültige Refreshes)
func Refresh(c *gin.Context) {

	au, err := authentication.ExtractTokenMetadata(authentication.RT, c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, err.Error()) // msg: "refresh token expired"
		return
	}

	// ist das RT noch gültig? (macht beim AT die Middleware)
	err = authentication.TokenValid(authentication.RT, c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, err.Error())
		return
	}

	// userID für die Ausstellung eines neues Token Pair
	userID, err := authentication.FetchAuth(au)
	if err != nil {
		c.JSON(http.StatusUnauthorized, err.Error())
		return
	}

	// Die /refresh Route könnte auch eine leere Antwort zurückgeben. Vielleicht ist die erneute/aktualisierte
	// Lieferung von <User> mal sinnvoll, bspw. für aktualisierte Sicherheitsinformationen oder einen Zähler etc.
	dbUser, err := env.userModel.GetUserByID(userID)
	if err != nil {
		// user does not exist - erneute Prüfung eigentlich kaum nötig, kann aber noch mehr Sicherheit geben :-)
		if err == models.ErrInvalidUser {
			c.JSON(http.StatusUnauthorized, models.ErrInvalidUser.Error())
			return
		}
		// "real" error
		c.JSON(http.StatusInternalServerError, nil) // make client say "please try again later"
		return
	}

	// falls zu viele RTs (Clients) für den User in Umlauf sind, alle löschen, sonst nur das aktuelle
	// die ATs werden stehen gelassen; diese Clients können also noch damit arbeiten
	// ein neuer Refresh wird dann aber nicht mehr gehen
	deleted, err := authentication.DeleteAuths(authentication.RT, userID, au.TokenUUID)
	if err != nil || deleted == 0 { //if anything goes wrong
		c.JSON(http.StatusUnauthorized, authentication.ErrUnauthorized.Error())
		return
	}

	// create, register & save pair of AT/RT
	err = authentication.CreateTokens(c, userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, err.Error())
		return
	}

	// passwort nicht erneut zurücksenden
	dbUser.Password = ""

	c.JSON(http.StatusOK, &dbUser)
}

// ChangePassword sets a new password
func ChangePassword(c *gin.Context) {

	var dbUser *models.User

	// default auth-check
	userID, err := authentication.Authenticate(c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, authentication.ErrUnauthorized.Error())
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
		c.Status(http.StatusUnprocessableEntity)
		return
	}

	// re-load user's profile to perform additional security checks
	dbUser, err = env.userModel.GetUserByID(userID)
	if err != nil {
		// user does not exist
		if err == models.ErrInvalidUser {
			c.JSON(http.StatusUnauthorized, authentication.ErrUnauthorized.Error()) // report auth error
			return
		}
		// "real" error
		c.Status(http.StatusInternalServerError) // make client say "please try again later"
		return
	}

	// as an extra-security measure, compare given user name with the one referenced in the cookie
	if data.LoginName != dbUser.LoginName {
		c.JSON(http.StatusForbidden, authentication.ErrUnauthorized.Error())
		return
	}

	// check the current password (again)
	granted := env.userModel.CheckPassword(data.CurrentPWD, *dbUser)
	if !granted {
		c.JSON(http.StatusForbidden, authentication.ErrUnauthorized.Error())
		return
	}

	// ToDo: Validate new PWD (or include that in SetPWD)
	err = env.userModel.SetPassword(dbUser.ID, data.NewPassword)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, models.ErrInvalidPassword.Error())
		return
	}
}
