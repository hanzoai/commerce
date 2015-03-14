package variant

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

	var variant models.ProductVariant

	if err := d.Get(id, &variant); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[API.Variant.Get] %v", err)
		c.JSON(500, gin.H{"status": "unable to find product variant"})
	} else {
		c.JSON(200, variant)
	}
}

func Add(c *gin.Context) {
	d := datastore.New(c)

	var variant models.ProductVariant

	json.Decode(c.Request.Body, &variant)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[Api.Variant.Add] JSON: %v", variant)

	key, err := d.Put("productvariant", &variant)
	if err != nil {
		ctx.Errorf("[Api.Variant.Add] %v", err)
		c.JSON(500, gin.H{"status": "unable to save product variant"})
	} else {
		variant.Id = key.Encode()
		c.JSON(200, variant)
	}
}

func Update(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var variant models.ProductVariant

	json.Decode(c.Request.Body, &variant)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[API.Variant.Update] JSON: %v", variant)

	key, err := d.Update(id, &variant)
	if err != nil {
		ctx.Errorf("[API.Variant.Update] %v", err)
		c.JSON(500, gin.H{"status": "unable to find product variant"})
	} else {
		variant.Id = key
		c.JSON(200, variant)
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
