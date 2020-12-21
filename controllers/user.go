package controllers

import (
	"fmt"
	"forza-garage/authentication"
	"forza-garage/helpers"
	"forza-garage/models"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// UserExists maybe used to validate new accounts while typing into the form
func UserExists(c *gin.Context) {

	data := struct {
		UserName string `json:"loginName"`
	}{}

	// short syntax (err "zentral" deklariert)
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusUnprocessableEntity, "invalid json")
		return
	}

	fmt.Println(data)
	b := env.userModel.UserExists(data.UserName)
	if b == true {
		c.JSON(http.StatusUnprocessableEntity, nil)
		return
	}

	c.JSON(http.StatusOK, nil)
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

	c.JSON(http.StatusOK, ID) //std-struktur?
}

// Login a user
func Login(c *gin.Context) {

	var (
		err       error
		givenUser models.User
		dbUser    *models.User
	)

	// short syntax (err "zentral" deklariert)
	if err = c.ShouldBindJSON(&givenUser); err != nil {
		c.JSON(http.StatusUnprocessableEntity, "invalid json")
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
		c.JSON(http.StatusUnauthorized, "unauthorized")
		return
	}

	// nur at löschen, rt bleibt bestehen
	deleted, delErr := authentication.DeleteAuth(au.TokenUUID)
	if delErr != nil || deleted == 0 {
		c.JSON(http.StatusUnauthorized, "unauthorized")
		return
	}

	// "Hard log-out" => AT, RT & Cookie löschen => auf allen Geräten ausloggen
	// auch für Testzwecke nützlich

	au, err = authentication.ExtractTokenMetadata(authentication.RT, c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, "unauthorized")
		return
	}

	// rt löschen
	deleted, delErr = authentication.DeleteAuth(au.TokenUUID)
	if delErr != nil || deleted == 0 {
		c.JSON(http.StatusUnauthorized, "unauthorized")
		return
	}

	// Cookie löschen
	_ = helpers.DelCookie(c, os.Getenv("JWTCK_NAME"))

	c.JSON(http.StatusOK, nil)
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
		c.JSON(http.StatusUnauthorized, "unauthorized")
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
