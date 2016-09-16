package referrer

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/types/client"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
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
		if ok, _ := referrer.Query(db).Filter("Client.Ip=", ref.Client.Ip).KeysOnly().First(); ok {
			ref.Duplicate = true
		}

		if err := ref.Create(); err != nil {
			http.Fail(c, 500, "Failed to create referrer", err)
		} else {
			c.Writer.Header().Add("Location", c.Request.URL.Path+"/"+ref.Id())
			api.Render(c, 201, ref)
		}
	}

	api.Route(router, args...)
}
