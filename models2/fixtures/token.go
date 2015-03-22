package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/token"
)

func Token(c *gin.Context) *token.Token {
	db := datastore.New(c)

	token := token.New(db)

	// Generate ShortId
	token.Generate()

	token.Email = "test@test.com"
	token.UserId = "fake"
	token.Put()

	return token
}
