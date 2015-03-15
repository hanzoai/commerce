package rest

import (
	"reflect"

	"github.com/gin-gonic/gin"

	"crowdstart.io/models/mixin"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

var routes = make(map[string][]string)

// Helper that sets up all normal routes
func Restify(router *gin.RouterGroup, entity mixin.Entity) {
	model := new(mixin.Model)
	model.Entity = entity
	kind := model.Kind()

	log.Debug("Generating routes for " + kind)

	Get(router, model)
	List(router, model)
	Add(router, model)
	Update(router, model)
	Delete(router, model)
}

func Get(router *gin.RouterGroup, model *mixin.Model) {
	entityType := reflect.Indirect(reflect.ValueOf(model.Entity)).Type()
	kind := model.Kind()

	handler := func(c *gin.Context) {
		id := c.Params.ByName("id")
		model.SetContext(c)
		model.SetKey(id)

		entity := reflect.New(entityType).Interface()

		if err := model.GetEntity(entity); err != nil {
			message := "Failed to retrieve " + kind
			log.Debug(message+": %v", err, c)
			c.JSON(500, gin.H{"status": message})
		} else {
			c.JSON(200, entity)
		}
	}

	router.GET("/"+kind+"/:id", handler)
}

func List(router *gin.RouterGroup, model *mixin.Model) {
	entityType := reflect.Indirect(reflect.ValueOf(model.Entity)).Type()
	kind := model.Kind()

	handler := func(c *gin.Context) {
		model.SetContext(c)

		// Create a slice
		slice := reflect.MakeSlice(reflect.SliceOf(entityType), 0, 0)

		// Create pointer to a slice value and set it to the slice
		ptr := reflect.New(slice.Type())
		ptr.Elem().Set(slice)

		models := ptr.Interface()

		if _, err := model.Query().GetAll(models); err != nil {
			message := "Failed to list " + kind
			log.Debug(message+": %v", err, c)
			c.JSON(500, gin.H{"status": message})
		} else {
			c.JSON(200, models)
		}
	}

	router.GET("/"+kind, handler)
	router.GET("/"+kind+"/", handler)
}

func Add(router *gin.RouterGroup, model *mixin.Model) {
	entityType := reflect.Indirect(reflect.ValueOf(model.Entity)).Type()
	kind := model.Kind()

	handler := func(c *gin.Context) {
		model.SetContext(c)
		model.SetKey(nil)

		entity := reflect.New(entityType).Interface()

		if err := model.PutEntity(entity); err != nil {
			message := "Failed to add " + kind
			log.Debug(message, err, c)
			c.JSON(500, gin.H{"status": message})
		} else {
			c.JSON(200, entity)
		}

	}
	router.POST("/"+kind, handler)
}

func Update(router *gin.RouterGroup, model *mixin.Model) {
	entityType := reflect.Indirect(reflect.ValueOf(model.Entity)).Type()
	kind := model.Kind()

	handler := func(c *gin.Context) {
		model.SetContext(c)
		id := c.Params.ByName("id")
		model.SetKey(id)

		entity := reflect.New(entityType).Interface()

		model.GetEntity(entity)
		json.Decode(c.Request.Body, entity)

		if err := model.PutEntity(entity); err != nil {
			message := "Failed to update " + kind
			log.Debug(message, err, c)
			c.JSON(500, gin.H{"status": message})
		} else {
			c.JSON(200, entity)
		}
	}

	router.PUT("/"+kind+"/:id", handler)
}

func Delete(router *gin.RouterGroup, model *mixin.Model) {
	kind := model.Kind()

	handler := func(c *gin.Context) {
		model.SetContext(c)
		id := c.Params.ByName("id")
		model.SetKey(id)
		model.Delete()
	}

	router.DELETE("/"+kind+"/:id", handler)
}
