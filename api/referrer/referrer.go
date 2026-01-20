package referrer

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/models/types/client"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
	"github.com/hanzoai/commerce/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := rest.New(referrer.Referrer{})

	api.Create = func(c *gin.Context) {
		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c))
		ref := referrer.New(db)

		// Decode response body to create new order
		if err := json.Decode(c.Request.Body, ref); err != nil {
			http.Fail(c, 400, "Failed decode request body", err)
			return
		}

		// Save client-related data about request used to create referrer
		ref.Client = client.New(c)

		// Check if this is blacklisted IP
		ref.Blacklisted = ref.Client.Blacklisted()

		// Check if any other referrers have been created with this IP address
		if _, ok, _ := referrer.Query(db).Filter("Client.Ip=", ref.Client.Ip).FirstKey(); ok {
			ref.Duplicate = true
		}

		if err := ref.Create(); err != nil {
			http.Fail(c, 500, "Failed to create referrer", err)
		} else {
			c.Writer.Header().Add("Location", c.Request.URL.Path+"/"+ref.Id())
			api.Render(c, 201, ref)
		}
	}

	api.Get = func(c *gin.Context) {
		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c))
		ref := referrer.New(db)

		id := c.Params.ByName(api.ParamId)

		if err := ref.GetById(id); err != nil {
			http.Fail(c, 404, "No Referrer found with id: "+id, err)
			return
		}

		if err := ref.LoadAffiliate(); err != nil {
			http.Fail(c, 500, "Referrer affiliate data could not be queries", err)
			return
		}

		api.Render(c, 200, ref)
	}

	api.Route(router, args...)
}
