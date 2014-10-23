package crowdstart

import (
	"appengine"
	"appengine/datastore"
	"github.com/gin-gonic/gin"
	"github.com/twinj/uuid"
	"log"
	"net/http"
	"time"
)

type LineItem struct {
	id, quantity int
	price        float32
	name         string
}

type Session struct {
	id                    string
	created, last_updated int64
}

type Cart struct {
	items        []LineItem
	last_updated int64
	id           string
}

func CheckSession(ctx *gin.Context) {
	log.Println("Checking session")
	c := appengine.NewContext(ctx.Request)
	
	if cookie, err := ctx.Request.Cookie("crowdstart_cart"); err != nil {
		id := uuid.NewV4().String()
		cookie := &http.Cookie{
			Name:    "crowdstart_cart",
			Value:   id,
			Path:    "/",
			Expires: time.Now().Add(24 * time.Hour),
		}
		http.SetCookie(ctx.Writer, cookie)
		log.Println("Added cookie")

		ts := time.Now().Unix()

		session := Session{
			id:           id,
			created:      ts,
			last_updated: ts,
		}

		key, _ := datastore.Put(c, datastore.NewKey(c, "session", id, 0, nil), &session)

		ctx.Set("session", session)
		ctx.Set("key", key)
		ctx.Next()
	} else {
		id := cookie.Value
		var session Session
		datastore.Get(c, datastore.NewKey(c, "session", id, 0, nil), &session)
		ctx.Set("session", session)
	}
}
