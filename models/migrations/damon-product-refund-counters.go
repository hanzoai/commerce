package migrations

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/util/counter"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("damon-product-refund-counters",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "damon")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		if ord.Test {
			return
		}

		if ord.Total != ord.Refunded {
			return
		}

		ctx := db.Context

		for _, item := range ord.Items {
			prod := product.New(ord.Datastore())
			if err := prod.GetById(item.ProductId); err != nil {
				log.Error("no product found %v", err, ctx)
			}
			for i := 0; i < item.Quantity; i++ {
				if err := counter.IncrProductRefund(ctx, prod, ord); err != nil {
					log.Error("product."+prod.Id()+".refunded error %v", err, db.Context)
				}
			}
		}
	},
)
