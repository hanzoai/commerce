package subscription

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/permission"
)

var subscriptionEndpoint = config.UrlFor("api", "/subscription/")

// hydrateOrg populates payment credentials from KMS onto the org.
func hydrateOrg(c *zip.Ctx, org *organization.Organization) {
	if v := c.Locals("kms"); v != nil {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
			}
		}
	}
}

func getSubscription(c *zip.Ctx) (*subscription.Subscription, error) {
	// Get organization for this user
	org := middleware.GetOrganization(c)

	// Set up the db with the namespaced context
	ctx := org.Namespaced(c.Context())
	db := datastore.New(ctx)

	// Create order that's properly namespaced
	sub := subscription.New(db)

	// Get order if an existing order was referenced
	if id := c.Param("subscriptionid"); id != "" {
		if err := sub.GetById(id); err != nil {
			return nil, err
		}
	}

	return sub, nil
}

func Subscribe(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	hydrateOrg(c, org)

	sub, _, err := subscribe(c, org)
	if err != nil {
		return http.Fail(c, 500, "Error during subscribe", err)
	}

	c.SetHeader("Location", subscriptionEndpoint+sub.Id())
	num, err := sub.NumberFromId()
	if err != nil {
		return http.Fail(c, 500, "Error during subscribe", err)
	}
	sub.Number = num

	return http.Render(c, 200, sub)
}

func GetSubscribe(c *zip.Ctx) error {
	sub, err := getSubscription(c)
	if err != nil {
		return http.Fail(c, 404, "No subscription found", err)
	}

	num, err := sub.NumberFromId()
	if err != nil {
		return http.Fail(c, 500, "Error during subscribe", err)
	}
	sub.Number = num
	return http.Render(c, 200, sub)
}

func UpdateSubscribe(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	hydrateOrg(c, org)
	sub, err := getSubscription(c)
	if err != nil {
		return http.Fail(c, 404, "No subscription found", err)
	}

	_, err = updateSubscribe(c, org, sub)
	if err != nil {
		return http.Fail(c, 500, "Error during subscribe", err)
	}

	num, err := sub.NumberFromId()
	if err != nil {
		return http.Fail(c, 500, "Error during subscribe", err)
	}
	sub.Number = num

	return http.Render(c, 200, sub)
}

func Unsubscribe(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	hydrateOrg(c, org)
	sub, err := getSubscription(c)
	if err != nil {
		return http.Fail(c, 404, "No subscription found", err)
	}

	_, err = unsubscribe(c, org, sub)
	if err != nil {
		return http.Fail(c, 500, "Error during subscribe", err)
	}

	num, err := sub.NumberFromId()
	if err != nil {
		return http.Fail(c, 500, "Error during subscribe", err)
	}
	sub.Number = num

	return http.Render(c, 200, sub)
}

func Route(router zip.Router, args ...zip.Handler) {
	api := router.Group("")
	api.Use(zip.H(func(c *zip.Ctx) error {
		c.SetHeader("Access-Control-Allow-Origin", "*")
		return c.Next()
	}))

	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	// Charge Payment API
	api.Post("/subscribe", publishedRequired, Subscribe)
	api.Get("/subscribe/:subscriptionid", publishedRequired, GetSubscribe)
	api.Patch("/subscribe/:subscriptionid", publishedRequired, UpdateSubscribe)
	api.Delete("/subscribe/:subscriptionid", publishedRequired, Unsubscribe)
}
