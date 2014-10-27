package variant

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

	var json models.ProductVariant

	if err := d.Get(id, &json); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[API.Variant.Get] %v", err)
		c.JSON(500, gin.H{"status": "unable to find product variant"})
	} else {
		c.JSON(200, json)
	}
}

func Add(c *gin.Context) {
	d := datastore.New(c)

	var json models.ProductVariant

	util.DecodeJson(c, &json)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[Api.Variant.Add] JSON: %v", json)

	key, err := d.Put("productvariant", &json)
	if err != nil {
		ctx.Errorf("[Api.Variant.Add] %v", err)
		c.JSON(500, gin.H{"status": "unable to save product variant"})
	} else {
		json.Id = key
		c.JSON(200, json)
	}
}

func Update(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var json models.ProductVariant

	util.DecodeJson(c, &json)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[API.Variant.Update] JSON: %v", json)

	key, err := d.Update(id, &json)
	if err != nil {
		ctx.Errorf("[API.Variant.Update] %v", err)
		c.JSON(500, gin.H{"status": "unable to find product variant"})
	} else {
		json.Id = key
		c.JSON(200, json)
	}
}

func Delete(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	if err := d.Delete(id); err != nil {
		c.JSON(500, gin.H{"status": "failed to delete product variant"})
	} else {
		c.JSON(200, gin.H{"status": "ok"})
	}
}
