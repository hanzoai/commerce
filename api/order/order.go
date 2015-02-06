package order

import (
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"github.com/gin-gonic/gin"
)

func Get(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var order models.Order

	if err := d.Get(id, &order); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[Api.Order.Get] %v", err)
		c.JSON(500, gin.H{"status": "unable to find order"})
	} else {
		c.JSON(200, order)
	}
}

func Add(c *gin.Context) {
	d := datastore.New(c)

	var order models.Order

	json.Decode(c.Request.Body, &order)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[Api.Order.Add] JSON: %v", order)

	key, err := d.Put("order", &order)
	if err != nil {
		ctx.Errorf("[Api.Order.Add] %v", err)
		c.JSON(500, gin.H{"status": "unable to save order"})
	} else {
		order.Id = key.Encode()
		c.JSON(200, order)
	}
}

func Update(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var order models.Order

	json.Decode(c.Request.Body, &order)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[Api.Order.Update] JSON: %v", order)

	key, err := d.Update(id, &order)
	if err != nil {
		ctx.Errorf("[Api.Order.Update] %v", err)
		c.JSON(500, gin.H{"status": "unable to find cart"})
	} else {
		order.Id = key
		c.JSON(200, order)
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
