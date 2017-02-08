package form

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/form"
	"hanzo.io/models/organization"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/types/client"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"

	mailchimp "hanzo.io/thirdparty/mailchimp/tasks"
)

var subscriberEndpoint = config.UrlFor("api", "/subscriber/")

func subscribe(c *gin.Context, db *datastore.Datastore, org *organization.Organization, f *form.Form) {
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
	if err := f.AddSubscriber(s); err != nil {
		if err == form.SubscriberAlreadyExists {
			http.Fail(c, 409, "Subscriber already exists", nil)
		}
		http.Fail(c, 500, "Failed to save subscriber to mailing list", err)
		return
	}

	// Add subscriber to Mailchimp
	if f.Mailchimp.Enabled {
		mailchimp.Subscriber.Call(db.Context, f.JSON(), s.JSON())
	}

	// Forward subscriber (if enabled)
	forward(ctx, org, f, s)

	// Success!
	c.Writer.Header().Add("Location", subscriberEndpoint+s.Id())
	http.Render(c, 201, s)
}
