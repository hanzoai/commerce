// Package producttaxonomy is the HTTP surface for the product-builder taxonomy:
// options (+values), categories (hierarchical), tags, types, and the
// return/refund reason lookups. All tenant-scoped via middleware.Namespace, so a
// caller only ever touches its own org's taxonomy.
package producttaxonomy

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/productcategory"
	"github.com/hanzoai/commerce/models/productoption"
	"github.com/hanzoai/commerce/models/productoptionvalue"
	"github.com/hanzoai/commerce/models/producttag"
	"github.com/hanzoai/commerce/models/producttype"
	"github.com/hanzoai/commerce/models/refundreason"
	"github.com/hanzoai/commerce/models/returnreason"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	// Options + their values (the variant builder's raw material).
	// The nested param reuses rest's derived ParamId ("productoptionid" =
	// kind + "id"), so the /values subroute shares the same wildcard slot as
	// the default /:productoptionid route. gin panics on sibling wildcards
	// with differing names, so a dash-free "productoptionid" here would panic
	// at wiring. The dash lives only in the internal param name, never in the
	// URL path a client sends.
	options := rest.New(productoption.ProductOption{})
	options.GET("/:productoptionid/values", namespaced, ListOptionValues)
	options.POST("/:productoptionid/values", namespaced, AddOptionValue)
	options.Route(router, args...)

	rest.New(productoptionvalue.ProductOptionValue{}).Route(router, args...)

	// Hierarchical categories. Same wildcard-slot reasoning as options above:
	// the children subroute reuses the derived "product-categoryid" ParamId.
	categories := rest.New(productcategory.ProductCategory{})
	categories.GET("/:product-categoryid/children", namespaced, ListCategoryChildren)
	categories.Route(router, args...)

	rest.New(producttag.ProductTag{}).Route(router, args...)
	rest.New(producttype.ProductType{}).Route(router, args...)

	// Reason lookups for the returns/refunds flows.
	rest.New(returnreason.ReturnReason{}).Route(router, args...)
	rest.New(refundreason.RefundReason{}).Route(router, args...)
}

// ListOptionValues returns the values belonging to a product option.
func ListOptionValues(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	optionId := c.Param("productoptionid")

	// Verify the option exists (scopes the 404 to this org's data).
	opt := productoption.New(db)
	if err := opt.GetById(optionId); err != nil {
		return http.Fail(c, 404, "No product option found with id: "+optionId, err)
	}

	values := make([]*productoptionvalue.ProductOptionValue, 0, 16)
	if _, err := productoptionvalue.Query(db).Filter("OptionId=", opt.Id()).GetAll(&values); err != nil {
		return http.Fail(c, 500, "Failed to list option values", err)
	}
	return http.Render(c, 200, values)
}

// AddOptionValue creates a value under a product option.
func AddOptionValue(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	optionId := c.Param("productoptionid")

	opt := productoption.New(db)
	if err := opt.GetById(optionId); err != nil {
		return http.Fail(c, 404, "No product option found with id: "+optionId, err)
	}

	v := productoptionvalue.New(db)
	if err := json.DecodeBytes(c.Body(), v); err != nil {
		return http.Fail(c, 400, "Failed to decode request body", err)
	}
	if v.Value == "" {
		return http.Fail(c, 400, "value is required", errors.New("missing value"))
	}
	v.OptionId = opt.Id() // the path option wins over any body value
	if err := v.Create(); err != nil {
		return http.Fail(c, 500, "Failed to create option value", err)
	}
	return http.Render(c, 201, v)
}

// ListCategoryChildren returns the direct children of a category.
func ListCategoryChildren(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	categoryId := c.Param("product-categoryid")

	parent := productcategory.New(db)
	if err := parent.GetById(categoryId); err != nil {
		return http.Fail(c, 404, "No product category found with id: "+categoryId, err)
	}

	children := make([]*productcategory.ProductCategory, 0, 16)
	if _, err := productcategory.Query(db).Filter("ParentId=", parent.Id()).GetAll(&children); err != nil {
		return http.Fail(c, 500, "Failed to list category children", err)
	}
	return http.Render(c, 200, children)
}
