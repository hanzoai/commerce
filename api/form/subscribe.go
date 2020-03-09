package form

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/email"
	"hanzo.io/log"
	"hanzo.io/models/form"
	"hanzo.io/models/organization"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/types/client"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/util/counter"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
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
			return
		}
		http.Fail(c, 500, "Failed to save subscriber to mailing list", err)
		return
	}

	// Increment subscribers
	counter.IncrSubscriber(ctx, time.Now())

	if err := counter.IncrSubscribers(ctx, org, f.Id(), time.Now()); err != nil {
		log.Error("IncrSubscriber Error: %v", err, c)
	}

	if f.EmailList.Enabled {
		email.Subscribe(ctx, f, s, org)
	}

	// Send welcome email
	if f.SendWelcome {
		email.SendSubscriberWelcome(ctx, org, s, f)
	}

	// Save user as customer in Mailchimp if configured
	if f.Mailchimp.APIKey != "" {
		// Create new mailchimp client
		client := mailchimp.New(ctx, org.Mailchimp)

		firstName := s.Metadata["firstName"].(string)
		if firstName == "" {
			firstName = s.Metadata["FNAME"].(string)
		}
		lastName := s.Metadata["lastName"].(string)
		if lastName == "" {
			lastName = s.Metadata["FNAME"].(string)
		}

		if err := client.SubscribeForm(f.Mailchimp.ListId, s.Email, firstName, lastName); err != nil {
			log.Error("Mailchimp Subscribe Error: %v", err, c)
		}
	}

	// Forward subscriber (if enabled)
	forward(ctx, org, f, s)

	// Success!
	c.Writer.Header().Add("Location", subscriberEndpoint+s.Id())
	http.Render(c, 201, s)
}
