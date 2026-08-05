package cart

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/cart"
	"github.com/hanzoai/commerce/thirdparty/mailchimp"
	"github.com/hanzoai/commerce/util/hashid"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

type SetReq struct {
	Quantity    int    `json:"quantity"`
	ProductId   string `json:"productId"`
	ProductSlug string `json:"productSlug"`
	VariantSKU  string `json:"variantSku"`
}

type CartResponse struct {
	Id string `json:"id"`
}

func Set(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("cartid")

	// Get cart, fail if it doesn't exist
	car := cart.New(db)
	if err := car.GetById(id); err != nil {
		return http.Fail(c, 404, "No cart found with id: "+id, err)
	}

	// Decode request
	req := SetReq{}
	if err := json.DecodeBytes(c.Body(), &req); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	var setId string

	// Try to figure out what sort of item we are setting
	var typ string
	if req.ProductId != "" {
		key, err := hashid.DecodeKey(db.Context, req.ProductId)
		if err != nil {
			return http.Fail(c, 400, "Failed to decode id", err)
		}
		setId = req.ProductId

		typ = key.Kind()
	} else if req.ProductSlug != "" {
		typ = "product"
		setId = req.ProductSlug
	} else if req.VariantSKU != "" {
		typ = "variant"
		setId = req.VariantSKU
	} else {
		return http.Fail(c, 400, "No product or variant specified", errors.New("No product or variant specified"))
	}

	// Update cart with new item quantity information
	if err := car.SetItem(db, setId, typ, req.Quantity); err != nil {
		return http.Fail(c, 400, "Failed to update line item", err)
	}

	if car.Mailchimp.CheckoutUrl == "" {
		car.Mailchimp.CheckoutUrl = org.Mailchimp.CheckoutUrl
	}

	// Update cart in datastore
	var res error
	if err := car.Update(); err != nil {
		res = http.Fail(c, 500, "Failed to update cart", err)
	} else {
		res = http.Render(c, 200, car)
	}

	// Determine store to use
	storeId := car.StoreId
	if storeId == "" {
		storeId = org.DefaultStore
	}

	// Update Mailchimp cart
	if car.UserId != "" || car.Email != "" {
		client := mailchimp.New(db.Context, org.Mailchimp)
		client.UpdateOrCreateCart(storeId, car)
	}

	return res
}

func Discard(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("cartid")

	// Get cart, fail if it doesn't exist
	car := cart.New(db)
	if err := car.GetById(id); err != nil {
		return http.Fail(c, 404, "No cart found with id: "+id, err)
	}

	car.Status = cart.Discarded

	// Update cart in datastore
	var res error
	if err := car.Update(); err != nil {
		res = http.Fail(c, 500, "Failed to update cart", err)
	} else {
		res = http.Render(c, 200, CartResponse{Id: car.Id()})
	}

	// Determine store to use
	storeId := car.StoreId
	if storeId == "" {
		storeId = org.DefaultStore
	}

	// Update Mailchimp cart
	if car.UserId != "" || car.Email != "" {
		client := mailchimp.New(db.Context, org.Mailchimp)
		client.DeleteCart(storeId, car)
	}

	return res
}

func create(r *rest.Rest) zip.Handler {
	return func(c *zip.Ctx) error {
		if !r.CheckPermissions(c, "create") {
			return nil
		}

		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c.Context()))
		car := cart.New(db)

		if err := json.DecodeBytes(c.Body(), car); err != nil {
			return r.Fail(c, 400, "Failed decode request body", err)
		}

		if car.Mailchimp.CheckoutUrl == "" {
			car.Mailchimp.CheckoutUrl = org.Mailchimp.CheckoutUrl
		}

		if err := car.Create(); err != nil {
			return r.Fail(c, 500, "Failed to create "+r.Kind, err)
		}

		// Determine store to use
		storeId := car.StoreId
		if storeId == "" {
			storeId = org.DefaultStore
		}

		// Create Mailchimp cart
		if car.UserId != "" || car.Email != "" {
			client := mailchimp.New(db.Context, org.Mailchimp)
			client.CreateCart(storeId, car)
		}

		c.SetHeader("Location", c.Path()+"/"+car.Id())
		return r.Render(c, 201, car)
	}
}

// Completely replaces an cart for given `id`.
func update(r *rest.Rest) zip.Handler {
	return func(c *zip.Ctx) error {
		if !r.CheckPermissions(c, "update") {
			return nil
		}

		id := c.Param(r.ParamId)

		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c.Context()))
		car := cart.New(db)

		// Try to retrieve key from datastore
		key, ok, err := car.IdExists(id)
		if !ok {
			return r.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
		}

		if err != nil {
			return r.Fail(c, 500, "Failed to retrieve key for "+id, err)
		}

		// Decode response body to create new cart
		if err := json.DecodeBytes(c.Body(), car); err != nil {
			return r.Fail(c, 400, "Failed decode request body", err)
		}

		if car.Mailchimp.CheckoutUrl == "" {
			car.Mailchimp.CheckoutUrl = org.Mailchimp.CheckoutUrl
		}

		// Use same key to save cart
		car.SetKey(key)

		// Replace whatever was in the datastore with our new updated cart
		var res error
		if err := car.Update(); err != nil {
			res = r.Fail(c, 500, "Failed to update "+r.Kind, err)
		} else {
			res = r.Render(c, 200, car)
		}

		// Determine store to use
		storeId := car.StoreId
		if storeId == "" {
			storeId = org.DefaultStore
		}

		// Update Mailchimp cart
		if car.UserId != "" || car.Email != "" {
			client := mailchimp.New(db.Context, org.Mailchimp)
			client.UpdateOrCreateCart(storeId, car)
		}

		return res
	}
}

// Partially updates pre-existing cart by given `id`.
func patch(r *rest.Rest) zip.Handler {
	return func(c *zip.Ctx) error {
		if !r.CheckPermissions(c, "patch") {
			return nil
		}

		id := c.Param(r.ParamId)

		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c.Context()))
		car := cart.New(db)

		err := car.GetById(id)

		if err != nil {
			return r.Fail(c, 404, "No "+r.Kind+" found with id: "+id, err)
		}

		if err := json.DecodeBytes(c.Body(), car); err != nil {
			return r.Fail(c, 400, "Failed decode request body", err)
		}

		if car.Mailchimp.CheckoutUrl == "" {
			car.Mailchimp.CheckoutUrl = org.Mailchimp.CheckoutUrl
		}

		var res error
		if err := car.Update(); err != nil {
			res = r.Fail(c, 500, "Failed to update "+r.Kind, err)
		} else {
			res = r.Render(c, 200, car)
		}

		// Determine store to use
		storeId := car.StoreId
		if storeId == "" {
			storeId = org.DefaultStore
		}

		// Update Mailchimp cart
		if car.UserId != "" || car.Email != "" {
			client := mailchimp.New(db.Context, org.Mailchimp)
			client.UpdateOrCreateCart(storeId, car)
		}

		return res
	}
}
