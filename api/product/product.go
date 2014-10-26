package product

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util"

)

func Get(c *gin.Context) {
	d  := datastore.New(c)
	id := c.Params.ByName("id")

	var json models.Product

	if err := d.Get(id, &json); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[Api.Product.Get] %v", err)
		c.JSON(500, gin.H{"status": "unable to find product"})
	} else {
		c.JSON(200, json)
	}
}

func Add(c *gin.Context) {
	d := datastore.New(c)

	var json models.Product

	util.DecodeJson(c, &json)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[Api.Product.Add] JSON: %v", json)

	key, err := d.Put("cart", &json)
	if err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[Api.Product.Add] %v", err)
		c.JSON(500, gin.H{"status": "unable to save cart"})
	} else {
		json.Id = key
		c.JSON(200, json)
	}
}

func Update(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var json models.Product

	util.DecodeJson(c, &json)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[Api.product.Update] JSON: %v", json)

	key, err := d.Update(id, &json)
	if err != nil {
		ctx.Errorf("[Api.Product.Update] %v", err)
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
