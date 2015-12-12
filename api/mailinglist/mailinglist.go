package mailinglist

import (
	"fmt"
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
	mandrill "crowdstart.com/thirdparty/mandrill/tasks"
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
	db.Context = ml.Db.Context
	ctx := db.Context

	// Get organization for mailinglist
	org := organization.New(db)
	org.GetById(ml.Key().Namespace())

	// Mailing list doesn't exist
	if err := ml.Get(); err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve mailing list '%v': %v", id, err), err)
		return
	}

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
		mailchimp.Subscriber.Call(db.Context, ml.JSON(), s.JSON())
	}

	// Forward contact information
	if ml.Forward.Enabled {
		toEmail := ml.Forward.Email
		toName := ml.Forward.Name
		fromEmail := "noreply@crowdstart.com"
		fromName := "Crowdstart"
		subject := "New submission for form " + ml.Name
		html := fmt.Sprintf("%v", s)
		mandrill.Send.Call(ctx, org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, html)
	}

	// Success!
	c.Writer.Header().Add("Location", subscriberEndpoint+s.Id())
	http.Render(c, 201, s)

}
