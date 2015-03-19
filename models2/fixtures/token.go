package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/token"
	"crowdstart.io/util/task"
)

var _ = task.Func("models2-fixtures-token", func(c *gin.Context) {
	db := datastore.New(c)

	token := token.New(db)

	// Generate ShortId
	token.Generate()

	token.Email = "test@test.com"
	token.UserId = "fake"
	token.Put()
})
