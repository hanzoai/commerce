package migration

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models/product"
	"crowdstart.io/util/log"
)

func suchTeesSetup(c *gin.Context) {
	c.Set("namespace", "suchtees") // set namespace for worker funcs
}

var _ = New("suchtees", suchTeesSetup,
	func(db *datastore.Datastore, key datastore.Key, product *product.Product) {
		log.Debug("product: %v", product)
	},
)
