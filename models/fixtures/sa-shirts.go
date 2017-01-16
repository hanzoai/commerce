package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/product"
	"crowdstart.com/models/types/currency"
)

var StonedShirts = New("stoned-shirts", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "stoned"
	org.GetOrCreate("Name=", org.Name)

	nsdb := datastore.New(org.Namespaced(org.Context()))

	datastore.RunInTransaction(nsdb.Context, func(nsdb *datastore.Datastore) error {
		// Create earphone product
		prod := product.New(nsdb)
		prod.Slug = "shirt-black-s"
		prod.GetOrCreate("Slug=", prod.Slug)
		prod.Name = "Stoned Shirt - Black - Small"
		prod.Description = "The cover, the myth, the legend. The Stoned shirt everyone has asked for."
		prod.Price = currency.Cents(3000)
		prod.Inventory = 9000
		prod.Preorder = false
		prod.Hidden = false
		prod.MustUpdate()
		// Create earphone product
		prod1 := product.New(nsdb)
		prod1.Slug = "shirt-black-m"
		prod1.GetOrCreate("Slug=", prod.Slug)
		prod1.Name = "Stoned Shirt - Black - Medium"
		prod1.Description = "The cover, the myth, the legend. The Stoned shirt everyone has asked for."
		prod1.Price = currency.Cents(3000)
		prod1.Inventory = 9000
		prod1.Preorder = false
		prod1.Hidden = false
		prod1.MustUpdate()
		// Create earphone product
		prod2 := product.New(nsdb)
		prod2.Slug = "shirt-black-l"
		prod2.GetOrCreate("Slug=", prod.Slug)
		prod2.Name = "Stoned Shirt - Black - Large"
		prod2.Description = "The cover, the myth, the legend. The Stoned shirt everyone has asked for."
		prod2.Price = currency.Cents(3000)
		prod2.Inventory = 9000
		prod2.Preorder = false
		prod2.Hidden = false
		prod2.MustUpdate()
		// Create earphone product
		prod3 := product.New(nsdb)
		prod3.Slug = "shirt-black-xl"
		prod3.GetOrCreate("Slug=", prod.Slug)
		prod3.Name = "Stoned Shirt - Black - XL"
		prod3.Description = "The cover, the myth, the legend. The Stoned shirt everyone has asked for."
		prod3.Price = currency.Cents(3000)
		prod3.Inventory = 9000
		prod3.Preorder = false
		prod3.Hidden = false
		prod3.MustUpdate()
		// Create earphone product
		prod4 := product.New(nsdb)
		prod4.Slug = "shirt-black-xxl"
		prod4.GetOrCreate("Slug=", prod.Slug)
		prod4.Name = "Stoned Shirt - Black - XXL"
		prod4.Description = "The cover, the myth, the legend. The Stoned shirt everyone has asked for."
		prod4.Price = currency.Cents(3000)
		prod4.Inventory = 9000
		prod4.Preorder = false
		prod4.Hidden = false
		prod4.MustUpdate()
		// Create earphone product
		prod5 := product.New(nsdb)
		prod5.Slug = "shirt-black-xxxl"
		prod5.GetOrCreate("Slug=", prod.Slug)
		prod5.Name = "Stoned Shirt - Black - XXXL"
		prod5.Description = "The cover, the myth, the legend. The Stoned shirt everyone has asked for."
		prod5.Price = currency.Cents(3000)
		prod5.Inventory = 9000
		prod5.Preorder = false
		prod5.Hidden = false
		prod5.MustUpdate()

		return nil
	}, datastore.TransactionOptions{XG: true})

	// datastore.RunInTransaction(db.Context, func(db *datastore.Datastore) error {
	// 	// Create earphone product
	// 	prod2 := product.New(nsdb)
	// 	prod2.Slug = "shirt-white"
	// 	prod2.GetOrCreate("Slug=", prod.Slug)
	// 	prod2.Name = ""
	// 	prod2.Description = "The cover, the myth, the legend. The Stoned shirt everyone has asked for."
	// 	prod2.Price = currency.Cents(3000)
	// 	prod2.Inventory = 9000
	// 	prod2.Preorder = false
	// 	prod2.Hidden = false
	// 	prod2.MustUpdate()

	// 	v2s := variant.New(nsdb)
	// 	v2s.ProductId = prod.Id()
	// 	v2s.SKU = prod.Slug + "-S"
	// 	v2s.Name = prod.Name + " - Small"
	// 	v2s.Available = true
	// 	v2s.MustUpdate()

	// 	v2m := variant.New(nsdb)
	// 	v2m.ProductId = prod.Id()
	// 	v2m.SKU = prod.Slug + "-M"
	// 	v2m.Name = prod.Name + " - Medium"
	// 	v2m.Available = true
	// 	v2m.MustUpdate()

	// 	v2l := variant.New(nsdb)
	// 	v2l.ProductId = prod.Id()
	// 	v2l.SKU = prod.Slug + "-L"
	// 	v2l.Name = prod.Name + " - Large"
	// 	v2l.Available = true
	// 	v2l.MustUpdate()

	// 	v2xl := variant.New(nsdb)
	// 	v2xl.ProductId = prod.Id()
	// 	v2xl.SKU = prod.Slug + "-XL"
	// 	v2xl.Name = prod.Name + " - X Large"
	// 	v2xl.Available = true
	// 	v2xl.MustUpdate()

	// 	v2xxl := variant.New(nsdb)
	// 	v2xxl.ProductId = prod.Id()
	// 	v2xxl.SKU = prod.Slug + "-XXL"
	// 	v2xxl.Name = prod.Name + " - XX Large"
	// 	v2xxl.Available = true
	// 	v2xxl.MustUpdate()

	// 	v2xxxl := variant.New(nsdb)
	// 	v2xxxl.ProductId = prod.Id()
	// 	v2xxxl.SKU = prod.Slug + "-XXXL"
	// 	v2xxxl.Name = prod.Name + " - XXX Large"
	// 	v2xxxl.Available = true
	// 	v2xxxl.MustUpdate()
	// }, datastore.TransactionOptions{XG: true})

	return org
})
