package form

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/mailinglist"
	"hanzo.io/models/organization"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/types/client"
	"hanzo.io/util/counter"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"

	mailchimp "hanzo.io/thirdparty/mailchimp/tasks"
)

var subscriberEndpoint = config.UrlFor("api", "/subscriber/")

func subscribe(c *gin.Context, db *datastore.Datastore, org *organization.Organization, ml *mailinglist.MailingList) {
	ctx := db.Context

	// Make sure Subscriber is created with the right context
	s := subscriber.New(db)

	// Decode response body for subscriber
	if err := json.Decode(c.Request.Body, s); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Store metadata about client
	s.Client = client.New(c)

	// Save subscriber to mailing list
	if err := ml.AddSubscriber(s); err != nil {
		if err == mailinglist.SubscriberAlreadyExists {
			http.Fail(c, 409, "Subscriber already exists", nil)
			return
		}
		http.Fail(c, 500, "Failed to save subscriber to mailing list", err)
		return
	}

	if err := counter.IncrSubscriber; err != nil {
		log.Error("IncrSubscriber Error: %v", err, c)
	}

	// Increment subscribers
	if err := counter.IncrSubscribers(ctx, org, ml.Id(), time.Now()); err != nil {
		log.Warn("Redis Error %s", err, ctx)
	}

	// Add subscriber to Mailchimp
	if ml.Mailchimp.Enabled {
		mailchimp.Subscribe.Call(db.Context, ml.JSON(), s.JSON())
	}

	// Forward subscriber (if enabled)
	forward(ctx, org, ml, s)

	// Success!
	c.Writer.Header().Add("Location", subscriberEndpoint+s.Id())
	http.Render(c, 201, s)
}
