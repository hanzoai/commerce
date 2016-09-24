package fixtures

import (
	"crowdstart.com/models/discount"
	"crowdstart.com/models/discount/rule"
	"crowdstart.com/models/discount/scope"
	"crowdstart.com/models/discount/target"
	"github.com/gin-gonic/gin"
)

var Discount = New("discount", func(c *gin.Context) *discount.Discount {
	// Get namespaced db
	db := getNamespaceDb(c)

	prod := Product(c)

	// Create discount rules for ludela
	dis := discount.New(db)
	dis.Name = "Bulk Discount"
	dis.GetOrCreate("Name=", dis.Name)
	dis.Scope.Type = scope.Product
	dis.Scope.ProductId = prod.Id()
	dis.Target.Type = target.Product
	dis.Target.ProductId = prod.Id()

	rule1 := discount.Rule{
		rule.Trigger{
			Quantity: rule.Quantity{
				Start: 2,
			},
		},
		rule.Action{
			Discount: rule.Discount{
				Flat: 5,
			},
		},
	}

	rule2 := discount.Rule{}
	rule2.Trigger.Quantity.Start = 3
	rule2.Action.Discount.Flat = 16

	dis.Rules = []discount.Rule{rule1, rule2}
	dis.MustUpdate()

	return dis
})
