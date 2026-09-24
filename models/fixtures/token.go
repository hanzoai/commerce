package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/token"
)

var Token = New("token", func(c *zip.Ctx) *token.Token {
	db := getNamespaceDb(c)

	token := token.New(db)
	token.Email = "test@test.com"
	token.GetOrCreate("Email=", token.Email)
	token.UserId = "fake"
	token.MustPut()

	return token
})
