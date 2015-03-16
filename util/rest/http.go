package rest

import (
	"sort"

	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/json"
	"crowdstart.io/util/template"
)

type byKind []mixin.Entity

func (e byKind) Len() int           { return len(e) }
func (e byKind) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
func (e byKind) Less(i, j int) bool { return e[i].Kind() < e[j].Kind() }

func DebugIndex(entities []mixin.Entity) gin.HandlerFunc {
	sort.Sort(byKind(entities))

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
		template.Render(c, "index.html",
			"orgId", org.Id(),
			"email", "dev@hanzo.ai",
			"password", "suchtees",
			"token", token,
			"entities", entities,
		)

		// Skip rest of handlers
		c.Abort(200)
	}
}
