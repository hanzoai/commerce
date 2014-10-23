package crowdstart

import (
	"appengine"
	"appengine/datastore"
	"github.com/gin-gonic/gin"
//	"github.com/twinj/uuid"
	"log"
	"net/http"
	"time"
)

type LineItem struct {
	id, quantity int
	price        float32
	name         string
}

type Cart struct {
	items                 []LineItem
	created, last_updated int64
}

func CheckSession(ctx *gin.Context) {
	log.Println("Checking session")
	c := appengine.NewContext(ctx.Request)

	if cookie, err := ctx.Request.Cookie("crowdstart_cart"); err != nil {
		//id := uuid.NewV4().String()
		ts := time.Now().Unix()

		cart := Cart{
			created:      ts,
			last_updated: ts,
		}

		key, _ := datastore.Put(c, datastore.NewIncompleteKey(c, "cart", nil), &cart)

		cookie := &http.Cookie{
			Name:    "crowdstart_cart",
			Value:   key.StringID(),
			Path:    "/",
			Expires: time.Now().Add(24 * time.Hour),
		}
		
		http.SetCookie(ctx.Writer, cookie)

		ctx.Set("cart", cart)
		ctx.Set("key", key)
		ctx.Next()
	} else {
		id := cookie.Value
		var cart Cart
		
		key := datastore.NewKey(c, "cart", id, 0, nil)
		datastore.Get(c, key, &cart)
		
		ctx.Set("cart", cart)
	}
}
