package rest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/reflect"
	"github.com/hanzoai/commerce/util/search"
)

var restApis = make([]*Rest, 0)

type route struct {
	url      string
	method   string
	handlers []zip.Handler
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
	Get              zip.Handler
	List             zip.Handler
	Create           zip.Handler
	Update           zip.Handler
	Patch            zip.Handler
	Delete           zip.Handler
	MethodOverride   zip.Handler

	middleware []zip.Handler
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
	// Param NAME must be dash-free: fiber parses ":product-optionid" as param
	// "product" + literal "-optionid" (a dash ends the param name), so a dashed
	// kind would register unmatchable routes. The URL prefix keeps the dashed
	// kind; only the param placeholder is squashed.
	r.ParamId = strings.ReplaceAll(r.Kind, "-", "") + "id"
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
	r.routes = make(routeMap)

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

// handle registers one route chain (middleware…, handler LAST) under the
// zip.Router method matching the HTTP verb — the single place the generic CRUD
// scaffold maps a method string onto zip's typed router surface.
func handle(group zip.Router, method, url string, handlers ...zip.Handler) {
	switch method {
	case "GET":
		group.Get(url, handlers...)
	case "POST":
		group.Post(url, handlers...)
	case "PUT":
		group.Put(url, handlers...)
	case "PATCH":
		group.Patch(url, handlers...)
	case "DELETE":
		group.Delete(url, handlers...)
	case "HEAD":
		group.Head(url, handlers...)
	case "OPTIONS":
		group.Options(url, handlers...)
	default:
		log.Panic("rest: unsupported method %q", method)
	}
}

func (r *Rest) Route(api zip.Router, mw ...zip.Handler) {
	prefix := r.Prefix + r.Kind
	prefix = "/" + strings.TrimLeft(prefix, "/")

	// Create group for our API routes
	group := api.Group(prefix)

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
		handle(group, route.method, route.url, append(mw, route.handlers...)...)
	}

	// Custom sub-routes keep their OWN handler chain (each already carries the
	// middleware it needs, e.g. an explicit Namespace). We deliberately do NOT
	// blanket-apply the base-CRUD `mw` here: the base middleware is the CRUD's
	// authz (often adminRequired), which is the WRONG gate for sub-routes meant to
	// be caller-scoped rather than admin-only (e.g. GET /user/:id/wallet) —
	// forcing it 401/403s legitimate access.
	//
	// Red HIGH-4 (money sub-routes reachable by non-admin) is closed at the
	// authoritative layer instead: every money-moving handler calls
	// middleware.RequireAdmin FIRST (IAM-aware, and fail-closed even when no
	// route-level token middleware ran) — giftcard Redeem/Void, checkout Refund,
	// b2b Accept/Reject/Approve, wallet Send, transaction Create/Hold, wire
	// Credit. That is stricter and more precise than propagating the base gate,
	// and it works on the IAM path where TokenRequired(Admin) no-ops.
	for _, routes := range r.routes {
		for _, route := range routes {
			// log.Debug("%-7s %v", route.method, prefix+route.url)
			handle(group, route.method, route.url, route.handlers...)
		}
	}
}

// CheckPermissions renders a 403 and returns false when the token lacks
// permission for method; the render is a side-effect (the response is written),
// so a denied handler just returns nil.
func (r Rest) CheckPermissions(c *zip.Ctx, method string) bool {
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
			handlers: []zip.Handler{r.Create},
		},
		route{
			method:   "GET",
			url:      "",
			handlers: []zip.Handler{r.List},
		},
		route{
			method:   "GET",
			url:      "/:" + r.ParamId,
			handlers: []zip.Handler{r.Get},
		},
		route{
			method:   "PUT",
			url:      "/:" + r.ParamId,
			handlers: []zip.Handler{r.Update},
		},
		route{
			method:   "DELETE",
			url:      "/:" + r.ParamId,
			handlers: []zip.Handler{r.Delete},
		},
		route{
			method:   "POST",
			url:      "/:" + r.ParamId,
			handlers: []zip.Handler{r.MethodOverride},
		},
		route{
			method:   "PATCH",
			url:      "/:" + r.ParamId,
			handlers: []zip.Handler{r.Patch},
		},
	}
}

func (r Rest) newKind() mixin.Kind {
	return reflect.New(r.entityType).Interface().(mixin.Kind)
}

// Returns a new interface of this entity type
func (r Rest) newEntity(c *zip.Ctx) mixin.Entity {
	ctx := c.Context()

	// Create a new entity bound to the CALLER ORG's own per-org store. Every
	// generic REST merchant model (product/order/user/store/collection/discount/
	// variant/…) is physically isolated per org: NewNamespaced routes reads AND
	// writes to db.Manager.Org(<caller org>) keyed by the namespace the auth
	// middleware resolved from the gateway/EdgeAuth-minted X-Org-Id (verified JWT
	// owner). Global/DefaultNamespace kinds (no namespace) fall back to the shared
	// default DB; an unresolvable namespace fails closed (no DB), never the pool.
	db := datastore.NewNamespaced(ctx)
	entity := reflect.New(r.entityType).Interface().(mixin.Entity)

	// Wire up mixin.BaseModel if the entity uses the legacy embedding.
	// Model[T]-based models are wired via Init() instead.
	// Note: embedded mixin.BaseModel fields are named "BaseModel" in Go reflection.
	val := reflect.Indirect(reflect.ValueOf(entity))
	baseModelType := reflect.TypeOf(mixin.BaseModel{})
	if field := val.FieldByName("BaseModel"); field.IsValid() && field.Type() == baseModelType {
		model := mixin.BaseModel{Db: db, Entity: entity}

		// Disable Put/Delete if in test mode
		if middleware.GetPermissions(c).Has(permission.Test) {
			model.Mock = false // force mock off due to testing issues
		}

		field.Set(reflect.ValueOf(model))
	}

	// Initialize entity (works for both BaseModel and Model[T] models)
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

func (r Rest) Render(c *zip.Ctx, status int, data interface{}) error {
	return http.Render(c, status, data)
}

func (r Rest) Fail(c *zip.Ctx, status int, message interface{}, err error) error {
	return http.Fail(c, status, message, err)
}

func (r Rest) get(c *zip.Ctx) error {
	if !r.CheckPermissions(c, "get") {
		return nil
	}

	id := c.Param(r.ParamId)

	entity := r.newEntity(c)

	if err := entity.GetById(id); err != nil {
		// TODO: When is this a 404?
		return r.Fail(c, 404, "Failed to get "+r.Kind, err)
	}
	return r.Render(c, 200, entity)
}

// list returns a page of this kind scoped to the caller's org.
//
// SECURITY — per-tenant isolation. listBasic reads the datastore
// `WHERE namespace = <ns>`, where <ns> is the namespace middleware.Namespace()
// derives from the authenticated, gateway/EdgeAuth-minted X-Org-Id (a
// JWT-verified `owner` claim). That is the SAME scoping get/create/update/delete
// already apply, so a caller lists ONLY its own org's rows. No search backend is
// wired (search.Open returns a no-op empty index — historically every list came
// back empty), so the datastore is the one and only list path.
//
// The namespace IS the owner filter, so require it explicitly here: a request
// with no resolved org namespace (DefaultNamespace/global kinds, or any route
// that did not pass Namespace()) is NEVER served an un-scoped full-table scan —
// that would cross tenants. Fail closed to an empty page; those kinds stay
// reachable by id via get.
func (r Rest) list(c *zip.Ctx) error {
	if !r.CheckPermissions(c, "list") {
		return nil
	}

	// Default sort order.
	sortField := c.Query("sort")
	if sortField == "" {
		sortField = r.DefaultSortField
	}

	pageStr := c.Query("page")
	displayStr := c.Query("display")
	limitStr := c.Query("limit")

	entity := r.newEntity(c)

	// Fail closed unless the request carries a concrete org namespace — the
	// exact value listBasic scopes every query by. Empty ⇒ no per-tenant scope,
	// so serve an empty page rather than risk crossing tenants.
	if nscontext.GetNamespace(entity.Context()) == "" {
		return r.Render(c, 200, Pagination{
			Page:    pageStr,
			Display: displayStr,
			Models:  r.newEntitySlice(0, 0),
			Count:   0,
			Facets:  [][]search.FacetResult{},
		})
	}

	return r.listBasic(c, entity, pageStr, displayStr, limitStr, sortField)
}

func (r Rest) listBasic(c *zip.Ctx, entity mixin.Entity, pageStr, displayStr, limitStr, sortField string) error {
	// Create query
	q := entity.Query().All().Order(sortField)

	var display int
	var err error

	// if we have pagination values, then trigger pagination calculations
	if displayStr != "" {
		if display, err = strconv.Atoi(displayStr); err == nil && display > 0 {
			q = q.Limit(display)
		} else {
			return r.Fail(c, 500, "'display' must be positive and non-zero.", err)
		}
	}

	if pageStr != "" && displayStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			q = q.Offset(display * (page - 1))
		} else {
			return r.Fail(c, 500, "'page' must be positive and non-zero.", err)
		}
	}

	entities := r.newEntitySlice(0, 0)
	if _, err := q.GetAll(entities); err != nil {
		return r.Fail(c, 500, "Failed to list "+r.Kind, err)
	}

	count, err := entity.Query().All().Count()
	if err != nil {
		return r.Fail(c, 500, "Could not count the models.", err)
	}

	if limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			count = limit
		}
	}

	return r.Render(c, 200, Pagination{
		Page:    pageStr,
		Display: displayStr,
		Models:  entities,
		Count:   count,
		Facets:  [][]search.FacetResult{},
	})
}

func (r Rest) create(c *zip.Ctx) error {
	if !r.CheckPermissions(c, "create") {
		return nil
	}

	entity := r.newEntity(c)

	if err := json.DecodeBytes(c.Body(), entity); err != nil {
		return r.Fail(c, 400, "Failed decode request body", err)
	}

	if err := entity.Create(); err != nil {
		return r.Fail(c, 500, "Failed to create "+r.Kind, err)
	}
	c.SetHeader("Location", c.Path()+"/"+entity.Id())
	return r.Render(c, 201, entity)
}

// Completely replaces an entity for given `id`.
func (r Rest) update(c *zip.Ctx) error {
	if !r.CheckPermissions(c, "update") {
		return nil
	}

	id := c.Param(r.ParamId)

	entity := r.newEntity(c)

	// Try to retrieve key from datastore
	key, ok, err := entity.IdExists(id)
	if !ok {
		if err != nil {
			return r.Fail(c, 500, "Failed to retrieve key for "+id, err)
		}

		return r.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
	}

	// Preserve original key
	entity.SetKey(key)

	// Decode response body to create new entity
	if err := json.DecodeBytes(c.Body(), entity); err != nil {
		return r.Fail(c, 400, "Failed decode request body", err)
	}

	// Replace whatever was in the datastore with our new updated entity
	if err := entity.Update(); err != nil {
		return r.Fail(c, 500, "Failed to update "+r.Kind, err)
	}
	return r.Render(c, 200, entity)
}

// Partially updates pre-existing entity by given `id`.
func (r Rest) patch(c *zip.Ctx) error {
	if !r.CheckPermissions(c, "patch") {
		return nil
	}

	id := c.Param(r.ParamId)

	entity := r.newEntity(c)
	err := entity.GetById(id)
	if err != nil {
		return r.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
	}

	if err := json.DecodeBytes(c.Body(), entity); err != nil {
		return r.Fail(c, 400, "Failed decode request body", err)
	}

	if err := entity.Update(); err != nil {
		return r.Fail(c, 500, "Failed to update "+r.Kind, err)
	}
	return r.Render(c, 200, entity)
}

// Deletes an entity by given `id`
func (r Rest) delete(c *zip.Ctx) error {
	if !r.CheckPermissions(c, "delete") {
		return nil
	}

	id := c.Param(r.ParamId)
	entity := r.newEntity(c)
	err := entity.GetById(id)
	if err != nil {
		return r.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
	}

	db := entity.Datastore()
	key := db.NewIncompleteKey("deleted", nil)
	if _, err := db.Put(key, entity); err != nil {
		return r.Fail(c, 500, "Failed to start deletion "+r.Kind, err)
	}

	if err := entity.Delete(); err != nil {
		return r.Fail(c, 500, "Failed to delete "+r.Kind, err)
	}
	c.SetHeader("Content-Type", "application/json")
	return c.Bytes(204, make([]byte, 0))
}

var methodOverride = middleware.MethodOverride()

// This should be handled by middleware
func (r Rest) methodOverride(c *zip.Ctx) error {

	// Override request method
	if err := methodOverride(c); err != nil {
		return err
	}

	switch c.Method() {
	case "PATCH":
		return r.Patch(c)
	case "POST":
		return r.Patch(c)
	case "PUT":
		return r.Update(c)
	case "DELETE":
		return r.Delete(c)
	default:
		return r.Fail(c, 405, "Method not allowed", errors.New("Method not allowed"))
	}
}

func (r *Rest) Handle(method, url string, handlers []zip.Handler) {
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

func (r *Rest) Use(handlers ...zip.Handler) {
	r.middleware = append(r.middleware, handlers...)
}

func (r *Rest) GET(url string, handlers ...zip.Handler) {
	r.Handle("GET", url, handlers)
}

func (r *Rest) POST(url string, handlers ...zip.Handler) {
	r.Handle("POST", url, handlers)
}

func (r *Rest) DELETE(url string, handlers ...zip.Handler) {
	r.Handle("DELETE", url, handlers)
}

func (r *Rest) PATCH(url string, handlers ...zip.Handler) {
	r.Handle("PATCH", url, handlers)
}

func (r *Rest) PUT(url string, handlers ...zip.Handler) {
	r.Handle("PUT", url, handlers)
}

func (r *Rest) HEAD(url string, handlers ...zip.Handler) {
	r.Handle("HEAD", url, handlers)
}

func (r *Rest) OPTIONS(url string, handlers ...zip.Handler) {
	r.Handle("OPTIONS", url, handlers)
}
