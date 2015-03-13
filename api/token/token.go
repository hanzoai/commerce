package token

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/token"
	"crowdstart.io/util/log"
)

// Retrieve a token from datastore
func Get(c *gin.Context) {
	db := datastore.New(c)
	id := c.Params.ByName("id")

	token := token.New(db)

	if err := token.Get(id); err != nil {
		message := "Failed to retrieve token from datastore"
		log.Debug(message, err, c)
		c.JSON(500, gin.H{"status": message})
	} else {
		c.JSON(200, token)
	}
}

// List tokens in datastore
func List(c *gin.Context) {
	db := datastore.New(c)

	tokens := make([]token.Token, 0)
	if _, err := token.New(db).Query().GetAll(&tokens); err != nil {
		message := "Failed to retrieve tokens from datastore"
		log.Debug(message, err, c)
		c.JSON(500, gin.H{"status": message})
	} else {
		c.JSON(200, tokens)
		// for i := 0; i < len(tokens); i++ {
		// 	tokens[i].Model = mixin.NewModel(db, tokens[i])
		// 	tokens[i].SetKey(keys[i])
		// }
		// c.JSON(200, tokens)
	}
}

// func Add(c *gin.Context) {
// 	ctx := middleware.GetAppEngine(c)
// 	d := datastore.New(c)

// 	user := new(models.User)

// 	errs := binding.Bind(c.Request, user)
// 	if errs.Handle(c.Writer) {
// 		ctx.Errorf("[Api.User.Add] %v", errs)
// 		return
// 	}
// 	ctx.Infof("[Api.User.Add] JSON: %v", user)

// 	key, err := d.Put("user", user)
// 	if err != nil {
// 		ctx.Errorf("[Api.User.Add] %v", err)
// 		c.JSON(500, gin.H{"status": "unable to save user"})
// 	} else {
// 		user.Id = key.Encode()
// 		c.JSON(200, user)
// 	}
// }

// func Update(c *gin.Context) {
// 	d := datastore.New(c)
// 	id := c.Params.ByName("id")

// 	var user models.User

// 	json.Decode(c.Request.Body, &user)
// 	ctx := middleware.GetAppEngine(c)
// 	ctx.Infof("[Api.User.Update] JSON: %v", user)

// 	key, err := d.Update(id, &user)
// 	if err != nil {
// 		ctx.Errorf("[Api.User.Update] %v", err)
// 		c.JSON(500, gin.H{"status": "unable to update user"})
// 	} else {
// 		user.Id = key
// 		c.JSON(200, user)
// 	}
// }

// func Delete(c *gin.Context) {
// 	d := datastore.New(c)
// 	id := c.Params.ByName("id")

// 	if err := d.Delete(id); err != nil {
// 		c.JSON(500, gin.H{"status": "failed to delete cart"})
// 	} else {
// 		c.JSON(200, gin.H{"status": "ok"})
// 	}
// }
