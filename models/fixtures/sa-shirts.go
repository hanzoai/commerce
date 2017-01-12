package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/models/namespace"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
)

var Stoned = New("stoned", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "stoned"
	org.GetOrCreate("Name=", org.Name)

	datastore.RunInTransaction(db.Context, func(db *datastore.Datastore) error {
		// Create earphone product
		prod := product.New(nsdb)
		prod.Slug = "shirt-black"
		prod.GetOrCreate("Slug=", prod.Slug)
		prod.Name = "Stoned Shirt - Black"
		prod.Description = "The cover, the myth, the legend. The Stoned shirt everyone has asked for."
		prod.Price = currency.Cents(3000)
		prod.Inventory = 9000
		prod.Preorder = false
		prod.Hidden = false
		prod.MustUpdate()

		vs := variant.New(nsdb)
		vs.ProductId = prod.Id()
		vs.SKU = prod.Slug + "-S"
		vs.Name = prod.Name + " - Small"
		vs.Available = true
		vs.MustUpdate()

		vm := variant.New(nsdb)
		vm.ProductId = prod.Id()
		vm.SKU = prod.Slug + "-M"
		vm.Name = prod.Name + " - Medium"
		vm.Available = true
		vm.MustUpdate()

		vl := variant.New(nsdb)
		vl.ProductId = prod.Id()
		vl.SKU = prod.Slug + "-L"
		vl.Name = prod.Name + " - Large"
		vl.Available = true
		vl.MustUpdate()

		vxl := variant.New(nsdb)
		vxl.ProductId = prod.Id()
		vxl.SKU = prod.Slug + "-XL"
		vxl.Name = prod.Name + " - X Large"
		vxl.Available = true
		vxl.MustUpdate()

		vxxl := variant.New(nsdb)
		vxxl.ProductId = prod.Id()
		vxxl.SKU = prod.Slug + "-XXL"
		vxxl.Name = prod.Name + " - XX Large"
		vxxl.Available = true
		vxxl.MustUpdate()

		vxxxl := variant.New(nsdb)
		vxxxl.ProductId = prod.Id()
		vxxxl.SKU = prod.Slug + "-XXXL"
		vxxxl.Name = prod.Name + " - XXX Large"
		vxxxl.Available = true
		vxxxl.MustUpdate()

	}, datastore.TransactionOptions{XG: true})

	datastore.RunInTransaction(db.Context, func(db *datastore.Datastore) error {
		// Create earphone product
		prod2 := product.New(nsdb)
		prod2.Slug = "shirt-white"
		prod2.GetOrCreate("Slug=", prod.Slug)
		prod2.Name = ""
		prod2.Description = "The cover, the myth, the legend. The Stoned shirt everyone has asked for."
		prod2.Price = currency.Cents(3000)
		prod2.Inventory = 9000
		prod2.Preorder = false
		prod2.Hidden = false
		prod2.MustUpdate()

		v2s := variant.New(nsdb)
		v2s.ProductId = prod.Id()
		v2s.SKU = prod.Slug + "-S"
		v2s.Name = prod.Name + " - Small"
		v2s.Available = true
		v2s.MustUpdate()

		v2m := variant.New(nsdb)
		v2m.ProductId = prod.Id()
		v2m.SKU = prod.Slug + "-M"
		v2m.Name = prod.Name + " - Medium"
		v2m.Available = true
		v2m.MustUpdate()

		v2l := variant.New(nsdb)
		v2l.ProductId = prod.Id()
		v2l.SKU = prod.Slug + "-L"
		v2l.Name = prod.Name + " - Large"
		v2l.Available = true
		v2l.MustUpdate()

		v2xl := variant.New(nsdb)
		v2xl.ProductId = prod.Id()
		v2xl.SKU = prod.Slug + "-XL"
		v2xl.Name = prod.Name + " - X Large"
		v2xl.Available = true
		v2xl.MustUpdate()

		v2xxl := variant.New(nsdb)
		v2xxl.ProductId = prod.Id()
		v2xxl.SKU = prod.Slug + "-XXL"
		v2xxl.Name = prod.Name + " - XX Large"
		v2xxl.Available = true
		v2xxl.MustUpdate()

		v2xxxl := variant.New(nsdb)
		v2xxxl.ProductId = prod.Id()
		v2xxxl.SKU = prod.Slug + "-XXXL"
		v2xxxl.Name = prod.Name + " - XXX Large"
		v2xxxl.Available = true
		v2xxxl.MustUpdate()
	}, datastore.TransactionOptions{XG: true})

})
