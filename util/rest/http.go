package rest

import (
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
	"crowdstart.com/util/template"
)

// Wrapped model, with a few display helpers
type endpoint struct {
	mixin.Model
	rest   *Rest
	id     string
	count  string
	prefix string
	kind   string
}

func newEndpoint(db *datastore.Datastore, r *Rest) *endpoint {
	endpoint := new(endpoint)
	endpoint.prefix = strings.TrimLeft(r.Prefix, "/")
	endpoint.rest = r
	endpoint.kind = r.Kind
	endpoint.Model = mixin.Model{Db: db, Entity: r.newKind()}
	return endpoint
}

func (e *endpoint) FirstId() string {
	if e.id == "" {
		if ok, _ := e.Model.Query().Get(); ok {
			e.id = e.Model.Id()
		} else {
			e.id = "<id>"
		}
	}

	return e.id
}

func (e *endpoint) EntityCount() string {
	if e.count == "" {
		count, _ := e.Query().All().Count()
		e.count = strconv.Itoa(count)
	}

	return e.count
}

func (e *endpoint) Url() string {
	return config.UrlFor("api", "/"+e.prefix+e.kind)
}

func (e *endpoint) UrlWithId() string {
	return e.Url() + "/" + e.FirstId()
}

type byKind []*Rest

func (e byKind) Len() int           { return len(e) }
func (e byKind) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
func (e byKind) Less(i, j int) bool { return e[i].Kind < e[j].Kind }

func ListRoutes() gin.HandlerFunc {
	sort.Sort(byKind(restApis))

	return func(c *gin.Context) {
		if !appengine.IsDevAppServer() {
			c.Next()
		}

		// Get default org
		db := datastore.New(c)
		org := organization.New(db)
		err := org.GetOrCreate("Name=", "suchtees")
		if err != nil {
			http.Fail(c, 500, "Unable to fetch organization", err)
			return
		}

		// Get namespaced datastore context
		orgDb := datastore.New(org.Namespaced(c))

		// We special case order endpoint because of a few useful API calls we want to work.
		var orderEndpoint *endpoint

		// Wrap models for display
		endpoints := make([]*endpoint, len(restApis))
		for i, r := range restApis {
			// Create fancy endpoint documentation for this API. If it has a
			// prefix of /c/, all calls should be made against the default
			// namespace, otherwise our fixture organization's namespace.
			if r.Prefix == "/c/" {
				endpoints[i] = newEndpoint(db, r)
			} else {
				endpoints[i] = newEndpoint(orgDb, r)
			}

			// Check if this is the order endpoint, if so we'll save a reference for later.
			if r.Kind == "order" {
				orderEndpoint = endpoints[i]
			}
		}

		token := middleware.GetAccessToken(c)

		log.Debug("fixture organization id: %v", org.Id())

		// Generate kind map
		template.Render(c, "index.html",
			"email", "dev@hanzo.ai",
			"endpoints", endpoints,
			"orderEndpoint", orderEndpoint,
			"organization", org,
			"password", "suchtees",
			"token", token,
		)
	}
}
