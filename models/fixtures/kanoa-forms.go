package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/form"
)

var _ = New("kanoa-forms", func(c *gin.Context) *mailinglist.MailingList {
	db := datastore.New(c)

	f := mailinglist.New(db)
	f.MustGetById("3XudPY2SQeXQ3")
	f.Forward.Name = "Cival"
	f.Forward.Email = "dev@hanzo.ai"
	f.Forward.Enabled = true

	return ml
})
