package helpers

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/chmike/securecookie"
	"github.com/gin-gonic/gin"
)

// https://github.com/chmike/securecookie

// ToDO: use .env
var cookieParams = securecookie.Params{
	Path:     "/",              // cookie received only when URL starts with this path
	Domain:   "",               // cookie received only when URL domain matches this one
	MaxAge:   3600 * 24 * 7,    // cookie becomes invalid 1 week after it is set (default 3600 seconds)
	HTTPOnly: true,             // disallow access by remote javascript code
	Secure:   false,            // cookie received only with HTTPS, never with HTTP
	SameSite: securecookie.Lax, // cookie received with same or sub-domain names
}

// SetCookie setzt ein Cookie
func SetCookie(c *gin.Context, name string, value interface{}) error {

	sck, err := securecookie.New(name, []byte(os.Getenv("JWTCK_HASHKEY")), cookieParams)
	if err != nil {
		return err
	}

	// erzeugt []byte
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}

	err = sck.SetValue(c.Writer, b)
	if err != nil {
		return err
	}

	return nil
}

// GetCookie liest ein Cookie
func GetCookie(r *http.Request, name string) (interface{}, error) {

	sck, err := securecookie.New(name, []byte(os.Getenv("JWTCK_HASHKEY")), cookieParams)
	if err != nil {
		return nil, err
	}

	val, err := sck.GetValue(nil, r)
	if err != nil {
		return nil, err
	}

	// []byte
	return val, nil
}

// DelCookie löscht ein Cookie
func DelCookie(c *gin.Context, name string) error {

	// delete cookie:
	// set a new cookie with the same name and negative MaxAge to delete it
	var cp = cookieParams
	cp.MaxAge = -1

	sck, err := securecookie.New(name, []byte(os.Getenv("JWTCK_HASHKEY")), cookieParams)
	if err != nil {
		return err
	}

	err = sck.Delete(c.Writer)
	if err != nil {
		return err
	}

	return nil
}
