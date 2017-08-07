package rest

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"appengine"
	aeds "appengine/datastore"
	"appengine/search"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/mixin"
	"hanzo.io/util/hashid"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"
	"hanzo.io/util/permission"
	"hanzo.io/util/reflect"
	"hanzo.io/util/router"
)

var restApis = make([]*Rest, 0)

type route struct {
	url      string
	method   string
	handlers []gin.HandlerFunc
}

type Opts struct {
	DefaultNamespace bool
	DefaultSortField string
}

type routeMap map[string](map[string]route)

type Rest struct {
	DefaultNamespace bool
	DefaultSortField string
	Kind             string
	ParamId          string
	Prefix           string
	Permissions      Permissions
	Get              gin.HandlerFunc
	List             gin.HandlerFunc
	Create           gin.HandlerFunc
	Update           gin.HandlerFunc
	Patch            gin.HandlerFunc
	Delete           gin.HandlerFunc
	MethodOverride   gin.HandlerFunc

	middleware []gin.HandlerFunc
	routes     routeMap

	entityType reflect.Type
	sliceType  reflect.Type
}

type Pagination struct {
	Page    string                 `json:"page,omitempty"`
	Display string                 `json:"display,omitempty"`
	Count   int                    `json:"count"`
	Models  interface{}            `json:"models"`
	Facets  [][]search.FacetResult `json:"facets"`
}

// These 3 facet structs are used for deserialization
type StringFacet struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type RangeFacet struct {
	Name  string `json:"name"`
	Value struct {
		Start float64 `json:"start"`
		End   float64 `json:"end"`
	} `json:"value"`
}

type Facets struct {
	StringFacets []StringFacet `json:"string"`
	RangeFacets  []RangeFacet  `json:"range"`
}

func (r *Rest) Init(prefix string) {
	r.Prefix = prefix
	r.routes = make(routeMap)
}

func (r *Rest) InitModel(entity mixin.Kind) {
	// Get type of entity
	r.entityType = reflect.ValueOf(entity).Type()
	ptrType := reflect.ValueOf(r.newKind()).Type()
	r.sliceType = reflect.SliceOf(ptrType)
	r.Kind = r.newKind().Kind()
	r.ParamId = r.Kind + "id"
	r.routes = make(routeMap)

	if r.DefaultSortField != "" {
		return
	}

	// Introspect model to determine default sort field
	for _, name := range reflect.FieldNames(entity) {
		if name == "Slug" || name == "SKU" {
			r.DefaultSortField = name
			return
		}
	}

	// Use Id_ as default sort field if nothing is specified.
	if r.DefaultSortField == "" {
		r.DefaultSortField = "UpdatedAt"
	}
}

func New(entityOrPrefix interface{}, args ...interface{}) *Rest {
	r := new(Rest)

	if len(args) > 0 {
		opts := args[0].(Opts)
		r.DefaultNamespace = opts.DefaultNamespace
		r.DefaultSortField = opts.DefaultSortField
	}

	switch v := entityOrPrefix.(type) {
	case string:
		r.Init(v)
	case mixin.Kind:
		r.InitModel(v)
		restApis = append(restApis, r) // Keep track of all APIs globally
	}

	return r
}

var Namespaced = middleware.Namespace()

func (r Rest) Route(router router.Router, mw ...gin.HandlerFunc) {
	prefix := r.Prefix + r.Kind
	prefix = "/" + strings.TrimLeft(prefix, "/")

	// Create group for our API routes
	group := router.Group(prefix)

	mw = append(r.middleware, mw...)

	if !r.DefaultNamespace {
		// Automatically namespace requests
		mw = append(mw, Namespaced)
	}

	// Setup default permissions
	if r.Permissions == nil {
		r.Permissions = DefaultPermissions[r.Kind]
	}

	// Add default routes
	for _, route := range r.defaultRoutes() {
		// log.Debug("%-7s %v", route.method, prefix+route.url)
		group.Handle(route.method, route.url, append(mw, route.handlers...)...)
	}

	for _, routes := range r.routes {
		for _, route := range routes {
			// log.Debug("%-7s %v", route.method, prefix+route.url)
			group.Handle(route.method, route.url, route.handlers...)
		}
	}
}

func (r Rest) CheckPermissions(c *gin.Context, method string) bool {
	// Get permissions of current token
	tok := middleware.GetPermissions(c)

	// Lookup permission
	permissions, ok := r.Permissions[method]

	// Unsupported method, need to define permissions
	if !ok {
		// TODO: Use more strict checks
		// msg := "Unsupported method for API access"
		// r.Fail(c, 500, msg, errors.New(msg))
		// return false
		msg := fmt.Sprintf("No permissions found matching method: '%s', skipping permission check.", method)
		log.Warn(msg, c)
		return true
	}

	// See if token matches any of the supported permissions
	for _, perm := range permissions {
		if tok.Has(perm) {
			return true
		}
	}

	// Token lacks valid permission
	msg := "Token lacks permission to " + method + " " + r.Kind
	r.Fail(c, 403, msg, errors.New(msg))
	return false
}

func (r Rest) defaultRoutes() []route {
	if r.Kind == "" {
		// Only supported on model APIs
		return []route{}
	}

	// Setup default handlers
	if r.Get == nil {
		r.Get = r.get
	}

	if r.List == nil {
		r.List = r.list
	}

	if r.Create == nil {
		r.Create = r.create
	}

	if r.Update == nil {
		r.Update = r.update
	}

	if r.Patch == nil {
		r.Patch = r.patch
	}

	if r.Delete == nil {
		r.Delete = r.delete
	}

	if r.MethodOverride == nil {
		r.MethodOverride = r.methodOverride
	}

	return []route{
		route{
			method:   "POST",
			url:      "",
			handlers: []gin.HandlerFunc{r.Create},
		},
		route{
			method:   "GET",
			url:      "",
			handlers: []gin.HandlerFunc{r.List},
		},
		route{
			method:   "GET",
			url:      "/:" + r.ParamId,
			handlers: []gin.HandlerFunc{r.Get},
		},
		route{
			method:   "PUT",
			url:      "/:" + r.ParamId,
			handlers: []gin.HandlerFunc{r.Update},
		},
		route{
			method:   "DELETE",
			url:      "/:" + r.ParamId,
			handlers: []gin.HandlerFunc{r.Delete},
		},
		route{
			method:   "POST",
			url:      "/:" + r.ParamId,
			handlers: []gin.HandlerFunc{r.MethodOverride},
		},
		route{
			method:   "PATCH",
			url:      "/:" + r.ParamId,
			handlers: []gin.HandlerFunc{r.Patch},
		},
	}
}

func (r Rest) newKind() mixin.Kind {
	return reflect.New(r.entityType).Interface().(mixin.Kind)
}

// Returns a new interface of this entity type
func (r Rest) newEntity(c *gin.Context) mixin.Entity {
	// Increase timeout
	ctx := middleware.GetAppEngine(c)
	ctx = appengine.Timeout(ctx, 15*time.Second)

	// Create a new entity
	db := datastore.New(ctx)
	entity := reflect.New(r.entityType).Interface().(mixin.Entity)
	model := mixin.Model{Db: db, Entity: entity}

	// Disable Put/Delete if in test mode
	if middleware.GetPermissions(c).Has(permission.Test) {
		model.Mock = false // force mock off due to testing issues
	}

	// Set model on entity
	field := reflect.Indirect(reflect.ValueOf(entity)).FieldByName("Model")
	field.Set(reflect.ValueOf(model))

	// Initialize entity
	entity.Init(db)

	return entity
}

// helper which returns a slice which is compatible with this entity
func (r Rest) newEntitySlice(length, capacity int) interface{} {
	// Create pointer to a slice value and set it to the slice
	slice := reflect.MakeSlice(r.sliceType, length, capacity)
	for i := 0; i < length; i++ {
		slice.Index(i).Set(reflect.New(r.entityType))
	}

	ptr := reflect.New(slice.Type())
	ptr.Elem().Set(slice)
	return ptr.Interface()
}

func (r Rest) Render(c *gin.Context, status int, data interface{}) {
	http.Render(c, status, data)
}

func (r Rest) Fail(c *gin.Context, status int, message interface{}, err error) {
	http.Fail(c, status, message, err)
}

func (r Rest) get(c *gin.Context) {
	if !r.CheckPermissions(c, "get") {
		return
	}

	id := c.Params.ByName(r.ParamId)

	entity := r.newEntity(c)

	if err := entity.GetById(id); err != nil {
		// TODO: When is this a 404?
		r.Fail(c, 404, "Failed to get "+r.Kind, err)
	} else {
		r.Render(c, 200, entity)
	}
}

func (r Rest) list(c *gin.Context) {
	log.Warn("list %v", r.Kind, c)
	if !r.CheckPermissions(c, "list") {
		return
	}

	query := c.Request.URL.Query()

	// Determine deafult sort order
	sortField := query.Get("sort")
	if sortField == "" {
		sortField = r.DefaultSortField
	}

	// Update query with page/display params
	pageStr := query.Get("page")
	displayStr := query.Get("display")
	limitStr := query.Get("limit")

	entity := r.newEntity(c)

	if _, ok := entity.(mixin.Searchable); ok {
		qStr := query.Get("q")
		fStr := query.Get("facets")
		r.listSearch(c, entity, qStr, fStr, pageStr, displayStr, limitStr, sortField)
	} else {
		r.listBasic(c, entity, pageStr, displayStr, limitStr, sortField)
	}
}

func (r Rest) listBasic(c *gin.Context, entity mixin.Entity, pageStr, displayStr, limitStr, sortField string) {
	// Create query
	q := entity.Query().All().Order(sortField)

	var display int
	var err error

	// if we have pagination values, then trigger pagination calculations
	if displayStr != "" {
		if display, err = strconv.Atoi(displayStr); err == nil && display > 0 {
			q = q.Limit(display)
		} else {
			r.Fail(c, 500, "'display' must be positive and non-zero.", err)
			return
		}
	}

	if pageStr != "" && displayStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			q = q.Offset(display * (page - 1))
		} else {
			r.Fail(c, 500, "'page' must be positive and non-zero.", err)
			return
		}
	}

	entities := r.newEntitySlice(0, 0)
	if _, err := q.GetAll(entities); err != nil {
		r.Fail(c, 500, "Failed to list "+r.Kind, err)
		return
	}

	count, err := entity.Query().All().Count()
	if err != nil {
		r.Fail(c, 500, "Could not count the models.", err)
		return
	}

	if limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			count = limit
		}
	}

	r.Render(c, 200, Pagination{
		Page:    pageStr,
		Display: displayStr,
		Models:  entities,
		Count:   count,
	})
}

func (r Rest) listSearch(c *gin.Context, entity mixin.Entity, qStr, fStr, pageStr, displayStr, limitStr, sortField string) {
	var display int
	var err error

	sortExpr := sortField
	sortReverse := sortExpr[0:1] == "-"
	if sortReverse {
		sortExpr = sortExpr[1:]
	}

	// should have already checked this
	opts := search.SearchOptions{}
	opts.Facets = []search.FacetSearchOption{
		search.AutoFacetDiscovery(100, 20),
	}
	opts.Sort = &search.SortOptions{
		Expressions: []search.SortExpression{
			search.SortExpression{
				Expr:    sortExpr,
				Reverse: sortReverse,
			},
		},
	}

	opts.Limit = 100

	// if we have pagination values, then trigger pagination calculations
	if displayStr != "" {
		if display, err = strconv.Atoi(displayStr); err == nil && display > 0 {
			opts.Limit = display
		} else {
			r.Fail(c, 500, "'display' must be positive and non-zero.", err)
			return
		}
	}

	if pageStr != "" && displayStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			opts.Offset = display * (page - 1)
		} else {
			r.Fail(c, 500, "'page' must be positive and non-zero.", err)
			return
		}
	}

	// open index
	index, err := search.Open(mixin.DefaultIndex)
	if err != nil {
		http.Fail(c, 500, fmt.Sprintf("Failed to open index for '"+r.Kind+"'"), err)
		return
	}

	keys := make([]*aeds.Key, 0)
	opts.IDsOnly = true
	opts.Refinements = []search.Facet{
		search.Facet{
			Name:  "Kind",
			Value: search.Atom(r.Kind),
		},
	}

	if fStr != "" {
		f := Facets{}
		if err := json.DecodeBytes([]byte(fStr), &f); err != nil {
			log.Warn("Unable to decode: %v", err, c)
		} else {
			for _, facet := range f.StringFacets {
				opts.Refinements = append(opts.Refinements, search.Facet{
					Name:  facet.Name,
					Value: search.Atom(facet.Value),
				})
			}
			for _, facet := range f.RangeFacets {
				opts.Refinements = append(opts.Refinements, search.Facet{
					Name: facet.Name,
					Value: search.Range{
						Start: facet.Value.Start,
						End:   facet.Value.End,
					},
				})
			}
		}
	}

	t := index.Search(entity.Context(), qStr, &opts)
	for {
		id, err := t.Next(nil) // We use the int id stored on the doc rather than the key
		if err == search.Done {
			break
		}
		if err != nil {
			http.Fail(c, 500, fmt.Sprintf("Failed to search index for '"+r.Kind+"'"), err)
			return
		}

		keys = append(keys, hashid.MustDecodeKey(entity.Context(), id))
	}

	facets, err := t.Facets()
	if err != nil {
		http.Fail(c, 500, fmt.Sprintf("Failed to get '"+r.Kind+"' options"), err)
		return
	}

	// Ignore this for now, use more accurate Kind facet count
	// t = index.Search(entity.Context(), qStr, &search.SearchOptions{
	// 	IDsOnly: true,
	// 	Refinements: []search.Facet{
	// 		search.Facet{
	// 			Name:  "Kind",
	// 			Value: search.Atom(r.Kind),
	// 		},
	// 	},
	// 	// CountAccuracy: 10000,
	// })
	// t.Next(entity.Context())
	// count := t.Count()
	count := 0

	entities := r.newEntitySlice(len(keys), len(keys))
	db := entity.Datastore()
	if err := db.GetMulti(keys, entities); err != nil {
		http.Fail(c, 500, fmt.Sprintf("Failed to get '"+r.Kind+"'"), err)
		return
	}

	if limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			count = limit
		}
	}

	if facets == nil {
		facets = [][]search.FacetResult{}
	}

	// Prevent +/-inf json 'unfortunate' serialization
	for i, facet := range facets {
		log.Error("Facet... %v", facet, c)
		for j, facetResult := range facet {
			if facetResult.Name == "Kind" {
				count = facetResult.Count
			}

			if r, ok := facetResult.Value.(search.Range); ok {
				s := r.Start
				if math.IsInf(s, -1) {
					s = -math.MaxFloat64
				}
				e := r.End
				if math.IsInf(e, 1) {
					e = math.MaxFloat64
				}
				facets[i][j].Value = search.Range{
					Start: s,
					End:   e,
				}
			}
		}
	}

	r.Render(c, 200, Pagination{
		Page:    pageStr,
		Display: displayStr,
		Models:  entities,
		Count:   count,
		Facets:  facets,
	})
}

func (r Rest) create(c *gin.Context) {
	if !r.CheckPermissions(c, "create") {
		return
	}

	entity := r.newEntity(c)

	if err := json.Decode(c.Request.Body, entity); err != nil {
		r.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if err := entity.Create(); err != nil {
		r.Fail(c, 500, "Failed to create "+r.Kind, err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path+"/"+entity.Id())
		r.Render(c, 201, entity)
	}
}

// Completely replaces an entity for given `id`.
func (r Rest) update(c *gin.Context) {
	if !r.CheckPermissions(c, "update") {
		return
	}

	id := c.Params.ByName(r.ParamId)

	entity := r.newEntity(c)

	// Try to retrieve key from datastore
	key, ok, err := entity.IdExists(id)
	if !ok {
		if err != nil {
			r.Fail(c, 500, "Failed to retrieve key for "+id, err)
			return
		}

		r.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
		return
	}

	// Preserve original key
	entity.SetKey(key)

	// Decode response body to create new entity
	if err := json.Decode(c.Request.Body, entity); err != nil {
		r.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Replace whatever was in the datastore with our new updated entity
	if err := entity.Update(); err != nil {
		r.Fail(c, 500, "Failed to update "+r.Kind, err)
	} else {
		r.Render(c, 200, entity)
	}
}

// Partially updates pre-existing entity by given `id`.
func (r Rest) patch(c *gin.Context) {
	if !r.CheckPermissions(c, "patch") {
		return
	}

	id := c.Params.ByName(r.ParamId)

	entity := r.newEntity(c)
	err := entity.GetById(id)
	if err != nil {
		r.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
		return
	}

	if err := json.Decode(c.Request.Body, entity); err != nil {
		r.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if err := entity.Update(); err != nil {
		r.Fail(c, 500, "Failed to update "+r.Kind, err)
	} else {
		r.Render(c, 200, entity)
	}
}

// Deletes an entity by given `id`
func (r Rest) delete(c *gin.Context) {
	if !r.CheckPermissions(c, "delete") {
		return
	}

	id := c.Params.ByName(r.ParamId)
	entity := r.newEntity(c)
	err := entity.GetById(id)
	if err != nil {
		r.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
		return
	}

	db := entity.Datastore()
	key := db.NewIncompleteKey("deleted", nil)
	if _, err := db.Put(key, entity); err != nil {
		r.Fail(c, 500, "Failed to start deletion "+r.Kind, err)
		return
	}

	if err := entity.Delete(); err != nil {
		r.Fail(c, 500, "Failed to delete "+r.Kind, err)
	} else {
		c.Data(204, "application/json", make([]byte, 0))
	}
}

var methodOverride = middleware.MethodOverride()

// This should be handled by middleware
func (r Rest) methodOverride(c *gin.Context) {

	// Override request method
	methodOverride(c)

	switch c.Request.Method {
	case "PATCH":
		r.Patch(c)
	case "POST":
		r.Patch(c)
	case "PUT":
		r.Update(c)
	case "DELETE":
		r.Delete(c)
	default:
		r.Fail(c, 405, "Method not allowed", errors.New("Method not allowed"))
	}
}

func (r Rest) Handle(method, url string, handlers []gin.HandlerFunc) {
	routes, ok := r.routes[url]
	if !ok {
		routes = make(map[string]route)
	}

	routes[method] = route{
		method:   method,
		url:      url,
		handlers: handlers,
	}

	r.routes[url] = routes
}

func (r Rest) Use(handlers ...gin.HandlerFunc) {
	r.middleware = append(r.middleware, handlers...)
}

func (r Rest) GET(url string, handlers ...gin.HandlerFunc) {
	r.Handle("GET", url, handlers)
}

func (r Rest) POST(url string, handlers ...gin.HandlerFunc) {
	r.Handle("POST", url, handlers)
}

func (r Rest) DELETE(url string, handlers ...gin.HandlerFunc) {
	r.Handle("DELETE", url, handlers)
}

func (r Rest) PATCH(url string, handlers ...gin.HandlerFunc) {
	r.Handle("PATCH", url, handlers)
}

func (r Rest) PUT(url string, handlers ...gin.HandlerFunc) {
	r.Handle("PUT", url, handlers)
}

func (r Rest) HEAD(url string, handlers ...gin.HandlerFunc) {
	r.Handle("HEAD", url, handlers)
}

func (r Rest) OPTIONS(url string, handlers ...gin.HandlerFunc) {
	r.Handle("OPTIONS", url, handlers)
}

func (r Rest) LINK(url string, handlers ...gin.HandlerFunc) {
	r.Handle("LINK", url, handlers)
}

func (r Rest) UNLINK(url string, handlers ...gin.HandlerFunc) {
	r.Handle("UNLINK", url, handlers)
}
