package token

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/token"
	"crowdstart.io/util/json"
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
	}
}

func Add(c *gin.Context) {
	db := datastore.New(c)

	token := token.New(db)

	json.Decode(c.Request.Body, token)

	if err := token.Put(); err != nil {
		message := "Failed to create token."
		log.Debug(message, err, c)
		c.JSON(500, gin.H{"status": message})
	} else {
		c.JSON(200, token)
	}
}

func Update(c *gin.Context) {
	db := datastore.New(c)
	id := c.Params.ByName("id")

	token := token.New(db)
	token.Get(id)

	json.Decode(c.Request.Body, token)

	if err := token.Put(); err != nil {
		message := "Failed to update token."
		log.Debug(message, err, c)
		c.JSON(500, gin.H{"status": message})
	} else {
		c.JSON(200, token)
	}
}

func Delete(c *gin.Context) {
	db := datastore.New(c)
	id := c.Params.ByName("id")

	token := token.New(db)
	token.Get(id)

	if err := token.Delete(); err != nil {
		message := "Failed to delete token."
		log.Debug(message, err, c)
		c.JSON(500, gin.H{"status": message})
	} else {
		c.JSON(200, gin.H{"status": "ok"})
	}
}
