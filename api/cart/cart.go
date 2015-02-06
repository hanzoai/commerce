package cart

import (
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"github.com/gin-gonic/gin"
	"github.com/mholt/binding"
)

func Get(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var cart models.Cart

	if err := d.Get(id, &cart); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("%v", err)
		c.JSON(500, gin.H{"status": "unable to find cart"})
	} else {
		c.JSON(200, cart)
	}
}

func Add(c *gin.Context) {
	d := datastore.New(c)

	cart := new(models.Cart)

	ctx := middleware.GetAppEngine(c)

	errs := binding.Bind(c.Request, cart)
	if errs.Handle(c.Writer) {
		ctx.Errorf("[Api.User.Add] %v", errs)
		return
	}
	ctx.Infof("[Api.Cart.Add] JSON: %v", cart)

	key, err := d.Put("cart", &cart)
	if err != nil {
		ctx.Errorf("[Api.Cart.Add] %v", err)
		c.JSON(500, gin.H{"status": "unable to save cart"})
	} else {
		cart.Id = key.Encode()
		c.JSON(200, cart)
	}
}

func Update(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var cart models.Cart

	json.Decode(c.Request.Body, &cart)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("JSON: %v", cart)

	key, err := d.Update(id, &cart)
	if err != nil {
		ctx.Errorf("%v", err)
		c.JSON(500, gin.H{"status": "unable to find cart"})
	} else {
		cart.Id = key
		c.JSON(200, cart)
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
