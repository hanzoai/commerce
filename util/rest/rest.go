package rest

import (
	"reflect"

	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

type Rest struct {
	Kind   string
	Get    gin.HandlerFunc
	List   gin.HandlerFunc
	Add    gin.HandlerFunc
	Update gin.HandlerFunc
	Delete gin.HandlerFunc

	namespace  bool
	entityType reflect.Type
}

func (r *Rest) Init(entity mixin.Entity) {
	r.Kind = entity.Kind()
	r.entityType = reflect.ValueOf(entity).Type()

	// Setup default handlers
	r.Get = r.get
	r.List = r.list
	r.Add = r.add
	r.Update = r.update
	r.Delete = r.delete
}

type Opts struct {
	NoNamespace bool
}

func New(entity mixin.Entity, args ...interface{}) *Rest {
	opts := Opts{}

	if len(args) > 0 {
		opts = args[0].(Opts)
	}

	r := new(Rest)
	r.Init(entity)

	// Options
	r.namespace = !opts.NoNamespace

	return r
}

func (r Rest) Route(router Router) {
	log.Debug("Registering routes for " + r.Kind)

	// Create group for our API routes and require Access token
	group := router.Group("/"+r.Kind, middleware.TokenRequired())

	// Add routes for defined handlers
	group.GET("", r.List)
	group.GET("/", r.List)
	group.GET("/:id", r.Get)
	group.POST("", r.Add)
	group.PUT("/:id", r.Update)
	group.DELETE("/:id", r.Delete)
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
	var ctx appengine.Context

	if r.namespace {
		ctx = middleware.GetAppEngine(c)
	} else {
		ctx = middleware.GetNamespace(c)
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
		message := "Failed to retrieve " + r.Kind
		log.Debug(message+": %v", err, c)
		r.JSON(c, 500, gin.H{"status": message})
	} else {
		r.JSON(c, 200, model.Entity)
	}
}

func (r Rest) list(c *gin.Context) {
	model := r.newModel(c)

	models := r.newEntitySlice()

	if _, err := model.Query().GetAll(models); err != nil {
		message := "Failed to list " + r.Kind
		log.Debug(message+": %v", err, c)
		r.JSON(c, 500, gin.H{"status": message})
	} else {
		r.JSON(c, 200, models)
	}
}

func (r Rest) add(c *gin.Context) {
	model := r.newModel(c)

	json.Decode(c.Request.Body, model.Entity)

	if err := model.Put(); err != nil {
		message := "Failed to add " + r.Kind
		log.Debug(message, err, c)
		r.JSON(c, 500, gin.H{"status": message})
	} else {
		r.JSON(c, 200, model.Entity)
	}
}

func (r Rest) update(c *gin.Context) {
	id := c.Params.ByName("id")

	model := r.newModel(c)
	model.Get(id)
	json.Decode(c.Request.Body, model.Entity)

	if err := model.Put(); err != nil {
		message := "Failed to update " + r.Kind
		log.Debug(message, err, c)
		r.JSON(c, 500, gin.H{"status": message})
	} else {
		r.JSON(c, 200, model.Entity)
	}
}

func (r Rest) delete(c *gin.Context) {
	id := c.Params.ByName("id")
	model := r.newModel(c)
	model.Delete(id)
}
