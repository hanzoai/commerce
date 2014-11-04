package auth

import (
	"github.com/gorilla/sessions"
	"github.com/gin-gonic/gin"
)

const sessionName = "crowdstartLogin"
const secret = "askjaakjl12"

var store = sessions.NewCookieStore([]byte(secret))

func IsLoggedIn(c *gin.Context) bool {
	session, err := store.Get(c.Request, sessionName)

	if err != nil{
		return false
	}

	return session.Values["id"] != nil
}

func Login(c *gin.Context, id string) error {
	session, err := store.Get(c.Request, sessionName)
	if err != nil {
		return err
	}
	session.Values["id"] = id
	return session.Save(c.Request, c.Writer)
}
