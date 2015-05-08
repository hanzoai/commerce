package store

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"

	mailchimp "crowdstart.com/thirdparty/mailchimp/tasks"
)

var subscriberEndpoint = config.UrlFor("api", "/subscriber/")

// Add subscriber to mailing list
func addSubscriber(c *gin.Context) {
	id := c.Params.ByName("mailinglistid")
	db := datastore.New(c)

	ml := mailinglist.New(db)

	// Set key and namespace correctly
	ml.SetKey(id)
	ml.SetNamespace(ml.Key().Namespace())

	if err := ml.Get(); err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve mailing list '%v': %v", id, err), err)
		return
	}

	// Make sure Subscriber is created with the right context
	db.Context = ml.Db.Context
	s := subscriber.New(db)

	// Decode response body for subscriber
	if err := json.Decode(c.Request.Body, s); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Save subscriber to mailing list
	if err := ml.AddSubscriber(s); err != nil {
		if err == mailinglist.SubscriberAlreadyExists {
			http.Fail(c, 409, "Subscriber already exists", nil)
		} else {
			http.Fail(c, 500, "Failed to save subscriber to mailing list", err)
		}
	} else {
		mailchimp.Subscriber.Call(db.Context, ml.JSON(), s.JSON())
		c.Writer.Header().Add("Location", subscriberEndpoint+s.Id())
		http.Render(c, 201, s)
	}
}

func js(c *gin.Context) {
	id := c.Params.ByName("mailinglistid")
	db := datastore.New(c)

	ml := mailinglist.New(db)

	// Set key and namespace correctly
	ml.SetKey(id)
	log.Debug("mailinglist: %v", ml)
	log.Debug("key: %v", ml.Key())
	log.Debug("namespace: %v", ml.Key().Namespace())
	ml.SetNamespace(ml.Key().Namespace())

	if err := ml.Get(); err != nil {
		c.String(404, fmt.Sprintf("Failed to retrieve mailing list '%v': %v", id, err))
		return
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")
	c.String(200, ml.Js())
}
