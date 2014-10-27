package cart

import (
	"github.com/gin-gonic/gin"
	"github.com/mholt/binding"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util"
)

func Get(c *gin.Context) {
	d  := datastore.New(c)
	id := c.Params.ByName("id")

	var json models.Cart

	if err := d.Get(id, &json); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("%v", err)
		c.JSON(500, gin.H{"status": "unable to find cart"})
	} else {
		c.JSON(200, json)
	}
}

func Add(c *gin.Context) {
	d := datastore.New(c)

	json := new(models.Cart)

	ctx := middleware.GetAppEngine(c)

	errs := binding.Bind(c.Request, json)
	if errs.Handle(c.Writer) {
		ctx.Errorf("[Api.User.Add] %v", errs)
		return
	}
	ctx.Infof("[Api.Cart.Add] JSON: %v", json)

	key, err := d.Put("cart", &json)
	if err != nil {
		ctx.Errorf("[Api.Cart.Add] %v", err)
		c.JSON(500, gin.H{"status": "unable to save cart"})
	} else {
		json.Id = key
		c.JSON(200, json)
	}
}

func Update(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var json models.Cart

	util.DecodeJson(c, &json)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("JSON: %v", json)

	key, err := d.Update(id, &json)
	if err != nil {
		ctx.Errorf("%v", err)
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
