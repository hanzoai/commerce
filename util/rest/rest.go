package rest

import (
	"errors"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

type route struct {
	url      string
	method   string
	handlers []gin.HandlerFunc
}

type Rest struct {
	DefaultNamespace bool
	Kind             string
	Get              gin.HandlerFunc
	List             gin.HandlerFunc
	Create           gin.HandlerFunc
	Update           gin.HandlerFunc
	Patch            gin.HandlerFunc
	Delete           gin.HandlerFunc
	Options          gin.HandlerFunc
	MethodOverride   gin.HandlerFunc

	routes     map[string]route
	entityType reflect.Type
}

type Pagination struct {
	Page    string      `json:"page,omitempty"`
	Display string      `json:"display,omitempty"`
	Count   int         `json:"count"`
	Models  interface{} `json:"models"`
}

func (r *Rest) Init(entity mixin.Entity) {
	r.Kind = entity.Kind()
	r.entityType = reflect.ValueOf(entity).Type()
	r.routes = make(map[string]route)
}

type Opts struct {
	DefaultNamespace bool
}

func New(entity mixin.Entity, args ...interface{}) *Rest {
	r := new(Rest)

	// Handle Options
	if len(args) > 0 {
		opts := args[0].(Opts)
		r.DefaultNamespace = opts.DefaultNamespace
	}
	r.Init(entity)

	return r
}

func (r Rest) Route(router Router, args ...gin.HandlerFunc) {
	// Create group for our API routes and require Access token
	group := router.Group("/" + r.Kind)
	group.Use(func(c *gin.Context) {
		c.Writer.Header().Add("Access-Control-Allow-Origin", "*")
	})
	group.Use(args...)

	// Add default routes
	for _, route := range r.defaultRoutes() {
		group.Handle(route.method, route.url, route.handlers)
	}

	for _, route := range r.routes {
		group.Handle(route.method, route.url, route.handlers)
	}
}

func (r Rest) defaultRoutes() []route {
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

// retuns a new interface of this entity type
func (r Rest) newEntity() interface{} {
	return reflect.New(r.entityType).Interface()
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

func (r Rest) newModel(c *gin.Context) mixin.Model {
	ctx := middleware.GetAppEngine(c)

	// Automatically use namespace of organization unless we're configured to
	// use the default namespace for this endpoint.
	if r.DefaultNamespace {
		log.Debug("Using default namespace.")
	} else {
		org := middleware.GetOrganization(c)
		ctx = org.Namespace(ctx)
		log.Debug("Using namespace: %d", org.Key().IntID())
	}

	db := datastore.New(ctx)
	entity := r.newEntity().(mixin.Entity)
	model := mixin.Model{Db: db, Entity: entity}
	return model
}

func (r Rest) JSON(c *gin.Context, code int, body interface{}) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(code)
	c.Writer.Write(json.EncodeBytes(body))
}

func (r Rest) get(c *gin.Context) {
	id := c.Params.ByName("id")

	model := r.newModel(c)

	if err := model.Get(id); err != nil {
		// TODO: When is this a 404?
		json.Fail(c, 404, "Failed to get "+r.Kind, err)
	} else {
		r.JSON(c, 200, model.Entity)
	}
}

func (r Rest) list(c *gin.Context) {
	model := r.newModel(c)

	models := r.newEntitySlice()

	query := c.Request.URL.Query()
	pageStr := query.Get("page")
	displayStr := query.Get("display")

	var display int
	var err error

	q := model.Query()

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

	if _, err = q.GetAll(models); err != nil {
		json.Fail(c, 500, "Failed to list "+r.Kind, err)
		return
	}

	count, err := model.Query().Count()
	if err != nil {
		json.Fail(c, 500, "Could not count the models.", err)
		return
	}

	r.JSON(c, 200, Pagination{
		Page:    pageStr,
		Display: displayStr,
		Models:  models,
		Count:   count,
	})
}

func (r Rest) create(c *gin.Context) {
	model := r.newModel(c)

	if err := json.Decode(c.Request.Body, model.Entity); err != nil {
		json.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if err := model.Put(); err != nil {
		json.Fail(c, 500, "Failed to create "+r.Kind, err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path+"/"+model.Id())
		r.JSON(c, 201, model.Entity)
	}
}

// Completely replaces an entity for given `id`.
func (r Rest) update(c *gin.Context) {
	id := c.Params.ByName("id")

	model := r.newModel(c)

	// Get Key, and fail if this didn't exist in datastore
	if _, err := model.GetKey(id); err != nil {
		json.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
		return
	}

	// Decode response body to create new entity
	if err := json.Decode(c.Request.Body, model.Entity); err != nil {
		json.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Replace whatever was in the datastore with our new updated entity
	if err := model.Put(); err != nil {
		json.Fail(c, 500, "Failed to update "+r.Kind, err)
	} else {
		r.JSON(c, 200, model.Entity)
	}
}

// Deletes an entity by given `id`
func (r Rest) delete(c *gin.Context) {
	id := c.Params.ByName("id")
	model := r.newModel(c)
	model.Delete(id)

	if err := model.Delete(); err != nil {
		json.Fail(c, 500, "Failed to delete "+r.Kind, err)
	} else {
		c.Data(204, "application/json", make([]byte, 0))
	}
}

// Partially updates pre-existing entity by given `id`.
func (r Rest) patch(c *gin.Context) {
	id := c.Params.ByName("id")

	model := r.newModel(c)
	err := model.Get(id)
	if err != nil {
		json.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
		return
	}

	if err := json.Decode(c.Request.Body, model.Entity); err != nil {
		json.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if err := model.Put(); err != nil {
		json.Fail(c, 500, "Failed to update "+r.Kind, err)
	} else {
		r.JSON(c, 200, model.Entity)
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
	reqMethods := c.Request.Header.Get("Access-Control-Request-Methods")
	reqHeaders := c.Request.Header.Get("Access-Control-Request-Headers")
	c.Writer.Header().Add("Access-Control-Allow-Methods", reqMethods)
	c.Writer.Header().Add("Access-Control-Allow-Headers", reqHeaders)
	c.Data(200, "text/plain", make([]byte, 0))
}

func (r Rest) Handle(method, url string, handlers []gin.HandlerFunc) {
	r.routes[url] = route{
		method:   method,
		url:      url,
		handlers: handlers,
	}
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
