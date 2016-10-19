package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mailinglist"
)

var _ = New("kanoa-forms", func(c *gin.Context) *mailinglist.MailingList {
	db := datastore.New(c)

	ml := mailinglist.New(db)
	ml.MustGetById("3XudPY2SQeXQ3")
	ml.Forward.Name = "Cival"
	ml.Forward.Email = "dev@hanzo.ai"
	ml.Forward.Enabled = true

	return ml
})
