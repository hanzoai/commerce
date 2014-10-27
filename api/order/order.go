package order

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util"
)

func Get(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var json models.Order

	if err := d.Get(id, &json); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[Api.Order.Get] %v", err)
		c.JSON(500, gin.H{"status": "unable to find order"})
	} else {
		c.JSON(200,json)
	}
}

func Add(c *gin.Context) {
	d := datastore.New(c)

	var json models.Order

	util.DecodeJson(c, &json)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[Api.Order.Add] JSON: %v", json)

	key, err := d.Put("order", &json)
	if err != nil {
		ctx.Errorf("[Api.Order.Add] %v", err)
		c.JSON(500, gin.H{"status": "unable to save order"})
	} else {
		json.Id = key
		c.JSON(200, json)
	}
}

func Update(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var json models.Order

	util.DecodeJson(c, &json)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[Api.Order.Update] JSON: %v", json)

	key, err := d.Update(id, &json)
	if err != nil {
		ctx.Errorf("[Api.Order.Update] %v", err)
		c.JSON(500, gin.H{"status": "unable to find cart"})
	} else {
		json.Id = key
		c.JSON(200, json)
	}
}

func Delete(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	if err := d.Delete(id); err != nil {
		c.JSON(500, gin.H{"status": "failed to delete cart"})
	} else {
		c.JSON(200, gin.H{"status": "ok"})
	}
}
