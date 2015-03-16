package rest

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/json"
	"crowdstart.io/util/template"
)

func DebugIndex(entities []mixin.Entity) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !appengine.IsDevAppServer() {
			c.Next()
		}

		db := datastore.New(c)
		org := organization.New(db)
		err := org.GetOrCreate("Name=", "suchtees")
		if err != nil {
			json.Fail(c, 500, "Unable to fetch organization", err)
			return
		}

		// Helper API page for dev
		query := c.Request.URL.Query()
		token := query.Get("token")

		// Generate kind map
		kinds := make(map[string]mixin.Entity, 0)
		for _, entity := range entities {
			kinds[entity.Kind()] = entity
		}

		template.Render(c, "index.html",
			"orgId", org.Id(),
			"email", "dev@hanzo.ai",
			"password", "suchtees",
			"token", token,
			"kinds", kinds,
		)

		// Skip rest of handlers
		c.Abort(200)
	}
}
