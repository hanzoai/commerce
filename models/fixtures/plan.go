package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/plan"
	"hanzo.io/models/types/currency"
)

var Plan = New("plan", func(c *gin.Context) *plan.Plan {
	// Get namespaced db
	db := getNamespaceDb(c)

	// Doge shirt
	pln := plan.New(db)
	pln.Slug = "much-shirts"
	pln.GetOrCreate("Slug=", pln.Slug)
	pln.Name = "Much Monthly Shirt"
	pln.Description = `wow
	　　　　　　such shirt
	much tee

			nice shop

	　so hip

	　　　　so doge
	`
	pln.Price = 2000
	pln.Currency = currency.USD
	pln.Interval = plan.Monthly
	pln.IntervalCount = 1
	// manually made in stripe test dashboard

	pln.MustPut()

	return pln
})
