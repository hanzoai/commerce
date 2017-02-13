package mailinglist

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/mailinglist"
	"hanzo.io/util/log"
)

func Js(c *gin.Context) {
	id := c.Params.ByName("mailinglistid")
	db := datastore.New(c)

	ml := mailinglist.New(db)

	// Set key and namespace correctly
	ml.SetKey(id)
	log.Debug("mailinglist: %v", ml, c)
	log.Debug("key: %v", ml.Key(), c)
	namespace := ml.Key().Namespace()
	log.Warn("namespace: %v", namespace, c)
	ml.SetNamespace(namespace)

	if err := ml.Get(nil); err != nil {
		log.Error("Failed to retrieve mailing list '%s' in namespace '%s': %v", id, namespace, err, c)
		c.String(404, fmt.Sprintf("Failed to retrieve mailing list '%v': %v", id, err))
		return
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")
	c.String(200, ml.Js())
}
