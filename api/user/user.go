package user

import (
	"github.com/gin-gonic/gin"
	"github.com/mholt/binding"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

func Get(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")
	log.Debug("ID: %v", id)

	var user models.User

	if err := d.Get(id, &user); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[Api.User.Get] %v", err)
		c.JSON(500, gin.H{"status": "unable to find user"})
	} else {
		c.JSON(200, user)
	}
}

func Add(c *gin.Context) {
	ctx := middleware.GetAppEngine(c)
	d := datastore.New(c)

	user := new(models.User)

	errs := binding.Bind(c.Request, user)
	if errs.Handle(c.Writer) {
		ctx.Errorf("[Api.User.Add] %v", errs)
		return
	}
	ctx.Infof("[Api.User.Add] JSON: %v", user)

	key, err := d.Put("user", user)
	if err != nil {
		ctx.Errorf("[Api.User.Add] %v", err)
		c.JSON(500, gin.H{"status": "unable to save user"})
	} else {
		user.Id = key.Encode()
		c.JSON(200, user)
	}
}

func Update(c *gin.Context) {
	d := datastore.New(c)
	id := c.Params.ByName("id")

	var user models.User

	json.Decode(c.Request.Body, &user)
	ctx := middleware.GetAppEngine(c)
	ctx.Infof("[Api.User.Update] JSON: %v", user)

	key, err := d.Update(id, &user)
	if err != nil {
		ctx.Errorf("[Api.User.Update] %v", err)
		c.JSON(500, gin.H{"status": "unable to update user"})
	} else {
		user.Id = key
		c.JSON(200, user)
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
