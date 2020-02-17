package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/log"
	"hanzo.io/models/organization"
	"hanzo.io/models/product"
	"hanzo.io/util/counter"

	aeds "google.golang.org/appengine/datastore"
	ds "hanzo.io/datastore"
)

func MustNukeCounter(db *ds.Datastore, tag string) {
	var ks []*aeds.Key
	var err error
	ks, err = db.Query(counter.ShardKind).Filter("Tag=", tag).Limit(500).KeysOnly().GetAll(nil)
	if err != nil {
		log.Panic("Cannot delete %s, %v", tag, err, db.Context)
	}
	for len(ks) != 0 {
		db.MustDeleteMulti(ks)
		ks, err = db.Query(counter.ShardKind).Filter("Tag=", tag).Limit(500).KeysOnly().GetAll(nil)
		if err != nil {
			log.Panic("Cannot delete %s, %v", tag, err, db.Context)
		}
	}
}

var _ = New("damon-reset-main-counters",
	func(c *gin.Context) []interface{} {
		orgName := "damon"

		c.Set("namespace", orgName)

		db := ds.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", orgName).Get(); err != nil {
			panic(err)
		}

		nsDb := ds.New(org.Namespaced(c))
		MustNukeCounter(nsDb, "order.projected.revenue")

		prods := make([]*product.Product, 0)
		if _, err := product.Query(nsDb).GetAll(&prods); err != nil {
			panic(err)
		}

		for _, prod := range prods {
			MustNukeCounter(nsDb, "product."+prod.Id()+".projected.revenue")
		}

		return NoArgs
	},
)
