package sessions

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

var store = sessions.NewCookieStore([]byte("ae5ZsJJ6ThySVPzkQM87KQSAtLfe67eU"))

func Get(c *gin.Context, name string) *sessions.Session {
	session, err := store.Get(c.Request, name)
	if err != nil {
		panic("Failed to get session")
	}
	return session
}
