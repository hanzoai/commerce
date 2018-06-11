package subscription

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/subscription"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/router"
)

var subscriptionEndpoint = config.UrlFor("api", "/subscription/")

func getSubscription(c *gin.Context) (*subscription.Subscription, error) {
	// Get organization for this user
	org := middleware.GetOrganization(c)

	// Set up the db with the namespaced appengine context
	ctx := org.Namespaced(c)
	db := datastore.New(ctx)

	// Create order that's properly namespaced
	sub := subscription.New(db)

	// Get order if an existing order was referenced
	if id := c.Params.ByName("subscriptionid"); id != "" {
		if err := sub.Get(id); err != nil {
			return nil, err
		}
	}

	return sub, nil
}

func Subscribe(c *gin.Context) {
	org := middleware.GetOrganization(c)

	sub, _, err := subscribe(c, org)
	if err != nil {
		http.Fail(c, 500, "Error during subscribe", err)
		return
	}

	c.Writer.Header().Add("Location", subscriptionEndpoint+sub.Id())
	sub.Number = sub.NumberFromId()
	http.Render(c, 200, sub)
}

func GetSubscribe(c *gin.Context) {
	sub, err := getSubscription(c)
	if err != nil {
		http.Fail(c, 404, "No subscription found", err)
		return
	}

	sub.Number = sub.NumberFromId()
	http.Render(c, 200, sub)
}

func UpdateSubscribe(c *gin.Context) {
	org := middleware.GetOrganization(c)
	sub, err := getSubscription(c)
	if err != nil {
		http.Fail(c, 404, "No subscription found", err)
		return
	}

	_, err = updateSubscribe(c, org, sub)
	if err != nil {
		http.Fail(c, 500, "Error during subscribe", err)
		return
	}

	sub.Number = sub.NumberFromId()
	http.Render(c, 200, sub)
}

func Unsubscribe(c *gin.Context) {
	org := middleware.GetOrganization(c)
	sub, err := getSubscription(c)
	if err != nil {
		http.Fail(c, 404, "No subscription found", err)
		return
	}

	_, err = unsubscribe(c, org, sub)
	if err != nil {
		http.Fail(c, 500, "Error during subscribe", err)
		return
	}

	sub.Number = sub.NumberFromId()
	http.Render(c, 200, sub)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("")
	api.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})

	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	// Charge Payment API
	api.POST("/subscribe", publishedRequired, Subscribe)
	api.GET("/subscribe/:subscriptionid", publishedRequired, GetSubscribe)
	api.POST("/subscribe/:subscriptionid", publishedRequired, UpdateSubscribe)
	api.DELETE("/subscribe/:subscriptionid", publishedRequired, Unsubscribe)
}
