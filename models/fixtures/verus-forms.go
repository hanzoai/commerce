package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/mailinglist"
)

var _ = New("verus-forms", func(c *context.Context) *mailinglist.MailingList {
	db := datastore.New(c)

	ml := mailinglist.New(db)
	ml.MustGetById("NEu14x75uv0Z6B")
	ml.Forward.Name = "Sales"
	ml.Forward.Email = "dev@hanzo.ai"
	ml.Forward.Enabled = true

	return ml
})
