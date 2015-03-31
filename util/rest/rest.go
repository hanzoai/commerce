package rest

import (
	"errors"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/router"
)

var restApis = make([]*Rest, 0)

type route struct {
	url      string
	method   string
	handlers []gin.HandlerFunc
}

type Opts struct {
	DefaultNamespace bool
}

type routeMap map[string](map[string]route)

type Rest struct {
	DefaultNamespace bool
	Kind             string
	Prefix           string
	Get              gin.HandlerFunc
	List             gin.HandlerFunc
	Create           gin.HandlerFunc
	Update           gin.HandlerFunc
	Patch            gin.HandlerFunc
	Delete           gin.HandlerFunc
	Options          gin.HandlerFunc
	MethodOverride   gin.HandlerFunc

	routes     routeMap
	entityType reflect.Type
}

type Pagination struct {
	Page    string      `json:"page,omitempty"`
	Display string      `json:"display,omitempty"`
	Count   int         `json:"count"`
	Models  interface{} `json:"models"`
}

func (r *Rest) Init(prefix string) {
	r.Prefix = prefix
	r.routes = make(routeMap)
}

func (r *Rest) InitModel(entity mixin.Kind) {
	// Get type of entity
	r.entityType = reflect.ValueOf(entity).Type()
	r.Kind = r.newKind().Kind()
	r.routes = make(routeMap)
}

func New(entityOrPrefix interface{}, args ...interface{}) *Rest {
	r := new(Rest)

	// Handle Options
	if len(args) > 0 {
		opts := args[0].(Opts)
		r.DefaultNamespace = opts.DefaultNamespace
	}

	switch v := entityOrPrefix.(type) {
	case string:
		r.Init(v)
	case mixin.Kind:
		r.InitModel(v)
		restApis = append(restApis, r) // Keep track of all apis globally
	}

	return r
}

func DefaultMiddleware(c *gin.Context) {
	// Ensure CORS works
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
}

func NamespacedMiddleware(c *gin.Context) {
	// Ensure CORS works
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Automatically use namespace of organization unless we're
	// configured to use the default namespace for this endpoint.
	ctx := middleware.GetAppEngine(c)
	org := middleware.GetOrganization(c)
	ctx = org.Namespace(ctx)
	c.Set("appengine", ctx)
}

func (r Rest) Route(router router.Router, args ...gin.HandlerFunc) {
	prefix := r.Prefix + r.Kind
	prefix = "/" + strings.TrimLeft(prefix, "/")

	log.Debug("Creating group with prefix: %v", prefix)
	// Create group for our API routes and require Access token
	group := router.Group(prefix)

	// Previous middleware should set organization on context, if non-default
	// namespace is used.
	group.Use(args...)

	var middleware gin.HandlerFunc

	// Setup our middleware
	if r.DefaultNamespace {
		middleware = DefaultMiddleware
	} else {
		middleware = NamespacedMiddleware
	}

	// Add default routes
	for _, route := range r.defaultRoutes() {
		log.Debug("Add route %v %v", route.method, prefix+route.url)
		handlers := append([]gin.HandlerFunc{middleware}, route.handlers...)
		group.Handle(route.method, route.url, handlers)
	}

	for _, routes := range r.routes {
		for _, route := range routes {
			log.Debug("Add route %v %v", route.method, prefix+route.url)
			group.Handle(route.method, route.url, route.handlers)
		}
	}
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

	if r.Options == nil {
		r.Options = r.options
	}

	return []route{
		route{
			method:   "OPTIONS",
			url:      "",
			handlers: []gin.HandlerFunc{r.Options},
		},
		route{
			method:   "OPTIONS",
			url:      "/*all",
			handlers: []gin.HandlerFunc{r.Options},
		},
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
			url:      "/",
			handlers: []gin.HandlerFunc{r.List},
		},
		route{
			method:   "GET",
			url:      "/:id",
			handlers: []gin.HandlerFunc{r.Get},
		},
		route{
			method:   "PUT",
			url:      "/:id",
			handlers: []gin.HandlerFunc{r.Update},
		},
		route{
			method:   "DELETE",
			url:      "/:id",
			handlers: []gin.HandlerFunc{r.Delete},
		},
		route{
			method:   "POST",
			url:      "/:id",
			handlers: []gin.HandlerFunc{r.MethodOverride},
		},
		route{
			method:   "PATCH",
			url:      "/:id",
			handlers: []gin.HandlerFunc{r.Patch},
		},
	}
}

func (r Rest) newKind() mixin.Kind {
	return reflect.New(r.entityType).Interface().(mixin.Kind)
}

// retuns a new interface of this entity type
func (r Rest) newEntity(c *gin.Context) mixin.Entity {
	// Create a new entity
	db := datastore.New(c)
	entity := reflect.New(r.entityType).Interface().(mixin.Entity)
	model := mixin.Model{Db: db, Entity: entity}

	// Disable Put/Delete if in test mode
	if middleware.GetPermissions(c).Has(permission.Test) {
		model.Mock = true
	}

	// Set model on entity
	field := reflect.Indirect(reflect.ValueOf(entity)).FieldByName("Model")
	field.Set(reflect.ValueOf(model))

	return entity
}

// helper which returns a slice which is compatible with this entity
func (r Rest) newEntitySlice() interface{} {
	// Create a slice
	slice := reflect.MakeSlice(reflect.SliceOf(r.entityType), 0, 0)

	// Create pointer to a slice value and set it to the slice
	ptr := reflect.New(slice.Type())
	ptr.Elem().Set(slice)

	return ptr.Interface()
}

func (r Rest) JSON(c *gin.Context, code int, body interface{}) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(code)
	c.Writer.Write(json.EncodeBytes(body))
}

func (r Rest) get(c *gin.Context) {
	id := c.Params.ByName("id")

	entity := r.newEntity(c)

	if err := entity.Get(id); err != nil {
		// TODO: When is this a 404?
		json.Fail(c, 404, "Failed to get "+r.Kind, err)
	} else {
		r.JSON(c, 200, entity)
	}
}

func (r Rest) list(c *gin.Context) {
	entity := r.newEntity(c)

	entities := r.newEntitySlice()

	query := c.Request.URL.Query()
	pageStr := query.Get("page")
	displayStr := query.Get("display")

	var display int
	var err error

	q := entity.Query()

	// if we have pagination values, then trigger pagination calculations
	if displayStr != "" {
		if display, err = strconv.Atoi(displayStr); err == nil && display > 0 {
			q = q.Limit(display)
		} else {
			json.Fail(c, 500, "'display' must be positive and non-zero.", err)
			return
		}
	}

	if pageStr != "" && displayStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			q = q.Offset(display * (page - 1))
		} else {
			json.Fail(c, 500, "'page' must be positive and non-zero.", err)
			return
		}
	}

	if _, err = q.GetAll(entities); err != nil {
		json.Fail(c, 500, "Failed to list "+r.Kind, err)
		return
	}

	count, err := entity.Query().Count()
	if err != nil {
		json.Fail(c, 500, "Could not count the models.", err)
		return
	}

	r.JSON(c, 200, Pagination{
		Page:    pageStr,
		Display: displayStr,
		Models:  entities,
		Count:   count,
	})
}

func (r Rest) create(c *gin.Context) {
	entity := r.newEntity(c)

	if err := json.Decode(c.Request.Body, entity); err != nil {
		json.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if err := entity.Put(); err != nil {
		json.Fail(c, 500, "Failed to create "+r.Kind, err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path+"/"+entity.Id())
		r.JSON(c, 201, entity)
	}
}

// Completely replaces an entity for given `id`.
func (r Rest) update(c *gin.Context) {
	id := c.Params.ByName("id")

	entity := r.newEntity(c)

	// Get Key, and fail if this didn't exist in datastore
	if _, err := entity.KeyExists(id); err != nil {
		json.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
		return
	}

	// Decode response body to create new entity
	if err := json.Decode(c.Request.Body, entity); err != nil {
		json.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Replace whatever was in the datastore with our new updated entity
	if err := entity.Put(); err != nil {
		json.Fail(c, 500, "Failed to update "+r.Kind, err)
	} else {
		r.JSON(c, 200, entity)
	}
}

// Partially updates pre-existing entity by given `id`.
func (r Rest) patch(c *gin.Context) {
	id := c.Params.ByName("id")

	entity := r.newEntity(c)
	err := entity.Get(id)
	if err != nil {
		json.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
		return
	}

	if err := json.Decode(c.Request.Body, entity); err != nil {
		json.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if err := entity.Put(); err != nil {
		json.Fail(c, 500, "Failed to update "+r.Kind, err)
	} else {
		r.JSON(c, 200, entity)
	}
}

// Deletes an entity by given `id`
func (r Rest) delete(c *gin.Context) {
	id := c.Params.ByName("id")
	entity := r.newEntity(c)
	entity.Delete(id)

	if err := entity.Delete(); err != nil {
		json.Fail(c, 500, "Failed to delete "+r.Kind, err)
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
		json.Fail(c, 405, "Method not allowed", errors.New("Method not allowed"))
	}
}

// Set proper CORS non-sense
func (r Rest) options(c *gin.Context) {
	header := c.Request.Header
	reqMethods := header.Get("Access-Control-Request-Methods")
	reqHeaders := header.Get("Access-Control-Request-Headers")
	header = c.Writer.Header()
	header.Set("Access-Control-Allow-Methods", reqMethods)
	header.Set("Access-Control-Allow-Headers", reqHeaders)
	c.Data(200, "text/plain", make([]byte, 0))
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
