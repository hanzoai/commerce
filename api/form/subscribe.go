package form

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/types/client"
	"crowdstart.com/util/counter"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"

	mailchimp "crowdstart.com/thirdparty/mailchimp/tasks"
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
