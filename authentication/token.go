package authentication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"forza-garage/helpers"
	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/twinj/uuid"
)

// token types
const (
	AT = "access_token"
	RT = "refresh_token"
)

// custom error types - evtl in eigenes file
var (
	ErrUnauthorized = errors.New("unauthorized") // invalid token/cookie
	ErrNotLoggedIn  = errors.New("requires authorization")
)

// TokenDetails enthält die Daten von AT und RT
type TokenDetails struct {
	AccessToken  string
	RefreshToken string
	AccessUUID   string
	RefreshUUID  string
	AtExpires    int64
	RtExpires    int64
}

// AccessDetails Token Metadata für die Registry (Key/Value redis)
type AccessDetails struct {
	TokenUUID string
	UserID    string
}

// CreateTokens erzeugt ein Token-Paar, regstriert es in Redis und sendet es via Cookie
func CreateTokens(c *gin.Context, userID string) error {

	// Create pair of AT & RT
	ts, err := CreateToken(userID)
	if err != nil {
		return err
	}

	// Register Tokens
	err = CreateAuth(userID, ts)
	if err != nil {
		return err
	}

	// Tokens für "Versendung" aufbereiten
	tokens := map[string]string{
		AT: ts.AccessToken,
		RT: ts.RefreshToken,
	}

	// Send token pair to client as a server-side cookie
	err = helpers.SetCookie(c, os.Getenv("JWTCK_NAME"), tokens)
	if err != nil {
		return err
	}

	return nil
}

// Authenticate prüft die Berechtigung zur Ausführung einer Route
// und liefert die UserID zurück
func Authenticate(r *http.Request) (string, error) {

	tokenAuth, err := ExtractTokenMetadata(AT, r)
	if err != nil {
		return "", err
	}

	userID, err := FetchAuth(tokenAuth)
	if err != nil {
		return "", err
	}

	return userID, nil
}

// DeleteAuths zählt die Tokens eines Benutzers. Wenn zu viele da sind (siehe Limit im Code)
// werden aus Sicherheitsgründen alle gelöscht; sonst nur das aktuelle/alte. Die Funktion
// wird üblicherweise für RTs benutzt, ist aber generisch gehalten
// Grundidee:
// https://valor-software.com/articles/json-web-token-authorization-with-access-and-refresh-tokens-in-angular-application-with-node-js-server.html
// alternativ können auch alle RTs gelöscht werden (param ggf. entfernen) und alle ATs falls >= x RTs vorhanden
// könnte halt mehr Last auslösen
func DeleteAuths(tokenType string, userID string, currentUUID string) (int64, error) {

	// redis is case-sensitive
	// tt := strings.ToLower(tokenType)
	tt := ""
	switch tokenType {
	case AT:
		tt = "at"
	case RT:
		tt = "rt"
	}
	searchString := tt + "_*"

	// fmt.Println("looking for ", searchString)

	var cursor uint64
	var allKeys []string // alle keys für den Token Type
	var usrKeys []string // alle keys für den gesuchten User

	var keys []string // gelesene keys für die cursor iteration
	var err error

	var ctx = context.Background()

	// in redis können nur keys durchsucht werden, nicht die values.
	// daher müssen zuerst alle keys für den User gelesen werden und dann werden deren
	// Inhalte client-seitig geprüft
	for {
		keys, cursor, err = client.Scan(ctx, cursor, searchString, 10).Result()
		if err != nil {
			return 0, err
		}

		// jede cursor iteration in die gesamtliste übernehmen
		allKeys = append(allKeys, keys...)

		/*
			// spread operator entspricht dem hier
			for _, v := range keys {
				allKeys = append(allKeys, v)
			}
		*/

		if cursor == 0 {
			break
		}
	}

	// fmt.Printf("found %d keys\n", len(allKeys))

	// values auslesen
	for _, v := range allKeys {
		//fmt.Println(i, v)

		val, err := client.Get(ctx, v).Result()
		if err != nil {
			panic(err)
		}

		if val == userID {
			usrKeys = append(usrKeys, v)
		}
	}

	// fmt.Printf("found %d for user\n", len(usrKeys))

	// delete all if there are X or more (r) tokens currently in use
	var delErr error // preserve last error that may have occured while in the loop
	var delCnt int64 = 0
	if len(usrKeys) >= 5 {
		// fmt.Println("deleting many auths")
		for _, v := range usrKeys {
			deleted, err := client.Del(ctx, v).Result()
			if err != nil {
				delErr = err
				delCnt = deleted
			}
			delCnt += deleted
		}
	} else {
		// sonst nur das aktuelle (r) token löschen
		if len(usrKeys) >= 1 {
			// fmt.Println("deleting current auth")
			delCnt, delErr = client.Del(ctx, currentUUID).Result()
		}
	}

	// in case of errors report 0 removals
	if delErr != nil {
		delCnt = 0
	}

	return delCnt, delErr
}

// CreateToken erzeugt ein Token-Paar (AT & RT)
func CreateToken(userID string) (*TokenDetails, error) {

	var err error
	td := &TokenDetails{}

	// access token
	td.AtExpires = time.Now().Add(time.Minute * 15).Unix() // default 15 min
	// td.AtExpires = time.Now().Add(time.Minute * 5).Unix() // test 5 min
	td.AccessUUID = "at_" + uuid.NewV4().String()

	// refresh token
	td.RtExpires = time.Now().Add(time.Hour * 24 * 7).Unix() // default 1 week
	// td.RtExpires = time.Now().Add(time.Minute * 10).Unix() // test 10 min
	td.RefreshUUID = "rt_" + uuid.NewV4().String()

	// create access token
	atClaims := jwt.MapClaims{}
	atClaims["authorized"] = true
	atClaims["access_uuid"] = td.AccessUUID
	atClaims["user_id"] = userID // userID rather than username (login name)
	atClaims["exp"] = td.AtExpires
	// weitere props analog https://github.com/omsec/racing-api/blob/master/login.php möglich

	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	td.AccessToken, err = at.SignedString([]byte(os.Getenv("ACCESS_SECRET")))
	if err != nil {
		return nil, err
	}

	// Create Refresh Token
	rtClaims := jwt.MapClaims{}
	rtClaims["refresh_uuid"] = td.RefreshUUID
	rtClaims["user_id"] = userID
	rtClaims["exp"] = td.RtExpires
	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	td.RefreshToken, err = rt.SignedString([]byte(os.Getenv("REFRESH_SECRET")))
	if err != nil {
		return nil, err
	}

	return td, nil
}

// CreateAuth speichert die Metadaten vom Token-Paar in der Registry (redis)
// erstmal öffentlich wegen refresh - noch pprüfen, ob der Aufruf in's CreateToken integriert werden soll
func CreateAuth(userID string, td *TokenDetails) error {

	var err error

	//converting Unix to UTC (to Time object)
	at := time.Unix(td.AtExpires, 0)
	rt := time.Unix(td.RtExpires, 0)
	now := time.Now()

	var ctx = context.Background()

	err = client.Set(ctx, td.AccessUUID, userID, at.Sub(now)).Err()
	if err != nil {
		return err
	}

	err = client.Set(ctx, td.RefreshUUID, userID, rt.Sub(now)).Err()
	if err != nil {
		return err
	}

	return nil
}

// ExtractToken liefert ein noch verschlüsseltes Token
func ExtractToken(tokenType string, r *http.Request) (string, error) {

	cval, err := helpers.GetCookie(r, os.Getenv("JWTCK_NAME"))
	if err != nil {
		return "", err
	}

	tokens := make(map[string]string)
	err = json.Unmarshal(cval.([]byte), &tokens)
	if err != nil {
		return "", err
	}

	return tokens[tokenType], nil
}

// VerifyToken prüft die Signatur
func VerifyToken(tokenType string, r *http.Request) (*jwt.Token, error) {

	tokenString, err := ExtractToken(tokenType, r)
	if err != nil {
		return nil, err // evtl. neuer Error
	}

	var secret []byte

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		//Make sure the token method conforms to "SigningMethodHMAC"
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		switch tokenType {
		case AT:
			secret = []byte(os.Getenv("ACCESS_SECRET"))
		case RT:
			secret = []byte(os.Getenv("REFRESH_SECRET"))

		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	return token, nil
}

// TokenValid prüft ob ein Token noch gültig ist
func TokenValid(tokenType string, r *http.Request) error {
	token, err := VerifyToken(tokenType, r)
	if err != nil {
		return err
	}
	if _, ok := token.Claims.(jwt.Claims); !ok && !token.Valid {
		return err
	}
	return nil
}

// ExtractTokenMetadata Metadata auslesen (für Redis-Zugriff)
func ExtractTokenMetadata(tokenType string, r *http.Request) (*AccessDetails, error) {

	var accessUUID string
	// map-extract funktion liefert ein boolean-flag statt error
	var ok bool

	token, err := VerifyToken(tokenType, r)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		// die UUID für das gewünschte Token auslesen
		switch tokenType {
		case AT:
			accessUUID, ok = claims["access_uuid"].(string)
			if !ok {
				return nil, err
			}
		case RT:
			accessUUID, ok = claims["refresh_uuid"].(string)
			if !ok {
				return nil, err
			}
		}
		userID, ok := claims["user_id"].(string)
		if !ok {
			return nil, err
		}
		return &AccessDetails{
			TokenUUID: accessUUID,
			UserID:    userID,
		}, nil
	}
	return nil, err
}

// FetchAuth liest die userID via Metada aus der Registry
func FetchAuth(authD *AccessDetails) (string, error) {

	var ctx = context.Background()
	userID, err := client.Get(ctx, authD.TokenUUID).Result()
	if err != nil {
		return "", err
	}
	return userID, nil
}

// DeleteAuth remove a token from the store upon log-out request
// (returns count of deleted records)
func DeleteAuth(givenUUID string) (int64, error) {

	var ctx = context.Background()
	deleted, err := client.Del(ctx, givenUUID).Result()
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

// TokenAuthMiddleware prüft das Token auf seine technische Gültigkeit
func TokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := TokenValid(AT, c.Request)
		if err != nil {
			//c.JSON(http.StatusUnauthorized, err.Error())
			c.JSON(http.StatusUnauthorized, ErrNotLoggedIn.Error())
			c.Abort()
			return
		}
		c.Next()
	}
}
