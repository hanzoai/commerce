package user

import (
	"github.com/gin-gonic/gin"
	"github.com/mholt/binding"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util"
)

func Get(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("Id")

	var json models.User

	if err := d.Get(id, &json); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[Api.User.Get] %v", err)
		c.JSON(500, gin.H{"status": "unable to find user"})
	} else {
		c.JSON(200, json)
	}
}

func Add(c *gin.Context) {
	ctx := middleware.GetAppEngine(c)
	d := datastore.New(c)

	json := new(models.User)

	errs := binding.Bind(c.Request, json)
	if errs.Handle(c.Writer) {
		ctx.Errorf("[Api.User.Add] %v", errs)
		return
	}
	ctx.Infof("[Api.User.Add] JSON: %v", json)

	key, err := d.Put("user", json)
	if err != nil {
		ctx.Errorf("[Api.User.Add] %v", err)
		c.JSON(500, gin.H{"status": "unable to save user"})
	} else {
		json.Id = key
		c.JSON(200, json)
	}
}

func Update(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var json models.User

	util.DecodeJson(c, &json)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[Api.User.Update] JSON: %v", json)

	key, err := d.Update(id, &json)
	if err != nil {
		ctx.Errorf("[Api.User.Update] %v", err)
		c.JSON(500, gin.H{"status": "unable to update user"})
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
