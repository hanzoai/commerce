package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/form"
)

var _ = New("kanoa-forms", func(c *gin.Context) *form.Form {
	db := datastore.New(c)

	f := form.New(db)
	f.MustGetById("3XudPY2SQeXQ3")
	f.Forward.Name = "Cival"
	f.Forward.Email = "dev@hanzo.ai"
	f.Forward.Enabled = true

	return f
})
