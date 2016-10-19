package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mailinglist"
)

var _ = New("verus-forms", func(c *gin.Context) *mailinglist.MailingList {
	db := datastore.New(c)

	ml := mailinglist.New(db)
	ml.MustGetById("NEu14x75uv0Z6B")
	ml.Forward.Name = "Sales"
	ml.Forward.Email = "dev@hanzo.ai"
	ml.Forward.Enabled = true

	return ml
})
