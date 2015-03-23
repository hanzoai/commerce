package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/token"
)

func Token(c *gin.Context) *token.Token {
	db := getDb(c)

	token := token.New(db)

	token.Email = "test@test.com"
	token.UserId = "fake"
	token.Put()

	return token
}
