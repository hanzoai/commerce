package rest

import (
	"reflect"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/organization"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// Wrapped model, with a few display helpers
type model struct {
	mixin.Model
	id    string
	count string
}

func newModel(db *datastore.Datastore, entity mixin.Entity) *model {
	// Create entity
	typ := reflect.ValueOf(entity).Type()
	e := reflect.New(typ).Interface().(mixin.Entity)
	// Create model
	m := new(model)
	// Embed entity, model
	m.Model = mixin.Model{Db: db, Entity: e}
	return m
}

func (m *model) DisplayId() string {
	if m.id == "" {
		if ok, _ := m.Model.Query().First(); ok {
			log.Debug("%#v", m.Model.Entity)
			m.id = m.Model.Id()
		} else {
			m.id = "<id>"
		}
	}

	return m.id
}

func (m *model) DisplayCount() string {
	if m.count == "" {
		count, _ := m.Query().Count()
		m.count = strconv.Itoa(count)
	}

	return m.count
}

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

		// Get default org
		db := datastore.New(c)
		org := organization.New(db)
		err := org.GetOrCreate("Name=", "suchtees")
		if err != nil {
			json.Fail(c, 500, "Unable to fetch organization", err)
			return
		}

		// Set datastore context to this org
		ctx, err := org.Namespace(c)
		if err != nil {
			json.Fail(c, 500, "Unable to set namespace.", err)
		}
		db = datastore.New(ctx)

		// Wrap models for display
		models := make([]*model, len(entities))
		for i, entity := range entities {
			models[i] = newModel(db, entity)
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
			"models", models,
		)

		// Skip rest of handlers
		c.Abort(200)
	}
}
