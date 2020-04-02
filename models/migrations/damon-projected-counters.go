package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/log"
	"hanzo.io/models/order"
	"hanzo.io/models/product"
	"hanzo.io/util/counter"

	ds "hanzo.io/datastore"
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
			prod := product.New(ord.Db)
			if err := prod.GetById(item.ProductId); err != nil {
				log.Error("no product found %v", err, ctx)
			}
			for i := 0; i < item.Quantity; i++ {
				if err := counter.IncrementByAll(ctx, "product."+prod.Id()+".projected.revenue", ord.StoreId, ord.ShippingAddress.Country, int(prod.ProjectedPrice), ord.CreatedAt); err != nil {
					log.Error("product."+prod.Id()+".projected.revenue error %v", err, db.Context)
				}
			}
		}
	},
)
