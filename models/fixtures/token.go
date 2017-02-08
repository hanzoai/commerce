package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/token"
)

var Token = New("token", func(c *gin.Context) *token.Token {
	db := getNamespaceDb(c)

	token := token.New(db)
	token.Email = "test@test.com"
	token.GetOrCreate("Email=", token.Email)
	token.UserId = "fake"
	token.MustPut()

	return token
})
