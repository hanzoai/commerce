package form

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/form"
	"hanzo.io/util/log"
)

func formJs(c *gin.Context) {
	id := c.Params.ByName("formid")
	db := datastore.New(c)

	ml := form.New(db)

	// Set key and namespace correctly
	ml.SetKey(id)
	log.Debug("form: %v", ml)
	log.Debug("key: %v", ml.Key())
	log.Debug("namespace: %v", ml.Key().Namespace())
	ml.SetNamespace(ml.Key().Namespace())

	if err := ml.Get(); err != nil {
		c.String(404, fmt.Sprintf("Failed to retrieve form '%v': %v", id, err))
		return
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")
	c.String(200, ml.Js())
}
