package store

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/price"
	"crowdstart.io/models2/product"
	"crowdstart.io/models2/variant"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

func variantPriceOverride(c *gin.Context, storeId, variantId string) *variant.Variant {
	// Get organization for this user
	org := middleware.GetOrganization(c)

	// Set up the db with the namespaced appengine context
	ctx := org.Namespace(c)
	db := datastore.New(ctx)

	// Create variant that's properly namespaced
	v := variant.New(db)
	if err := v.Get(variantId); err != nil {
		json.Fail(c, 404, "Failed to get "+v.Kind(), err)
	}

	p := price.New(db)
	ok, _ := p.Query().Filter("StoreId=", storeId).Filter("VariantId=", variantId).First()
	if ok {
		v.Price = p.Price
		v.Currency = p.Currency
	}

	return v
}

func productPriceOverride(c *gin.Context, storeId, productId string) *product.Product {
	// Get organization for this user
	org := middleware.GetOrganization(c)

	// Set up the db with the namespaced appengine context
	ctx := org.Namespace(c)
	db := datastore.New(ctx)

	// Create product that's properly namespaced
	prod := product.New(db)
	if err := prod.Get(productId); err != nil {
		json.Fail(c, 404, "Failed to get "+prod.Kind(), err)
	}

	p := price.New(db)
	ok, _ := p.Query().Filter("StoreId=", storeId).Filter("ProductId=", productId).First()
	log.Warn("OK? %v storeId %v productId %v", ok, storeId, productId)
	if ok {
		prod.Price = p.Price
		prod.Currency = p.Currency
	}

	return prod
}
