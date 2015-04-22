package store

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models2/mailinglist"
	"crowdstart.io/models2/subscriber"
	"crowdstart.io/util/json"
)

var subscriberEndpoint = config.UrlFor("api", "/subscriber/")

// Add subscriber to mailing list
func addSubscriber(c *gin.Context) {
	id := c.Params.ByName("mailinglistid")
	db := datastore.New(c)

	ml := mailinglist.New(db)

	if err := ml.Get(id); err != nil {
		json.Fail(c, 404, fmt.Sprintf("Failed to retrieve mailing list '%v': %v", id, err), err)
		return
	}

	s := subscriber.New(db)

	// Decode response body for subscriber
	if err := json.Decode(c.Request.Body, s); err != nil {
		json.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Save subscriber to mailing list
	if err := ml.AddSubscriber(s); err != nil {
		json.Fail(c, 500, "Failed to save subscriber to mailing list", err)
	} else {
		c.Writer.Header().Add("Location", subscriberEndpoint+s.Id())
		c.JSON(201, s)
	}
}

func js(c *gin.Context) {
	id := c.Params.ByName("mailinglistid")
	db := datastore.New(c)

	ml := mailinglist.New(db)

	if err := ml.Get(id); err != nil {
		c.String(404, fmt.Sprintf("Failed to retrieve mailing list '%v': %v", id, err))
		return
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")
	c.String(200, ml.Js())
}
