package checkout

import (
	"errors"
	nethttp "net/http"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/checkout/ethereum"
	"github.com/hanzoai/commerce/api/checkout/wire"
	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/permission"
)

var orderEndpoint = config.UrlFor("api", "/order/")

// authedOrg returns the authenticated caller's organization — set by the route's
// TokenRequired branch (service token / legacy org-bound token) or the IAM
// identity middleware — with its payment credentials hydrated from KMS. It is the
// ONE org source for every checkout money handler: the org is always the
// validated principal's, never client-supplied. Returns ok=false when no
// authenticated org is present so callers fail closed (401).
func authedOrg(c *zip.Ctx) (*organization.Organization, bool) {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return nil, false
	}

	// Hydrate payment credentials from KMS.
	if v := c.Locals("kms"); v != nil {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
			}
		}
	}
	return org, true
}

func getOrganizationAndOrder(c *zip.Ctx) (*organization.Organization, *order.Order, error) {
	// Org is the authenticated principal's (fail closed if the auth middleware
	// set none) — never client-supplied.
	org, ok := authedOrg(c)
	if !ok {
		err := errors.New("no authenticated organization")
		http.Fail(c, 401, "Authentication required", err)
		return nil, nil, err
	}

	// Set up the db with the namespaced context. NewNamespaced routes to the
	// caller org's OWN physical store (per-org SQLite via db.Manager.Org) so an
	// order — and every money move on it — is physically isolated per tenant,
	// never the shared systemDB/Postgres pool (Red CRIT-2).
	ctx := org.Namespaced(c.Context())
	db := datastore.NewNamespaced(ctx)

	// Create order that's properly namespaced
	ord := order.New(db)

	// Get order if an existing order was referenced
	if id := c.Param("orderid"); id != "" {
		if err := ord.GetById(id); err != nil {
			http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
			return nil, nil, OrderDoesNotExist
		}
	}

	return org, ord, nil
}

func Authorize(c *zip.Ctx) error {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		return nil // getOrganizationAndOrder already wrote the 401/404 response
	}

	if _, err = authorize(c, org, ord); err != nil {
		log.Error("Error %v %v", err.Error(), err, c)
		return http.Fail(c, 400, err.Error(), err)
	}

	log.JSON(ord)
	c.SetHeader("Location", orderEndpoint+ord.Id())
	log.JSON(ord)
	return http.Render(c, 200, ord)
}

func Capture(c *zip.Ctx) error {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		return nil // getOrganizationAndOrder already wrote the 401/404 response
	}

	if err = capture(c, org, ord); err != nil {
		log.Error("Error during capture %v", err, c)
		return http.Fail(c, 400, "Error during capture", err)
	}

	c.SetHeader("Location", orderEndpoint+ord.Id())
	return http.Render(c, 200, ord)
}

func Charge(c *zip.Ctx) error {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		return nil // getOrganizationAndOrder already wrote the 401/404 response
	}

	// Do authorization
	if _, err = authorize(c, org, ord); err != nil {
		log.Error("Error %v %v", err.Error(), err, c)
		return http.Fail(c, 400, "Error during authorize", err)
	}

	// Do capture using order from authorization
	if err = capture(c, org, ord); err != nil {
		log.Error("Error during capture %v", err, c)
		return http.Fail(c, 400, "Error during capture", err)
	}

	c.SetHeader("Location", orderEndpoint+ord.Id())
	return http.Render(c, 200, ord)
}

func Cancel(c *zip.Ctx) error {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		return nil // getOrganizationAndOrder already wrote the 401/404 response
	}

	if err := cancel(c, org, ord); err != nil {
		return http.Fail(c, 400, err.Error(), err)
	}

	return http.Render(c, 200, ord)
}

func Confirm(c *zip.Ctx) error {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		return nil // getOrganizationAndOrder already wrote the 401/404 response
	}

	if err := confirm(c, org, ord); err != nil {
		return http.Fail(c, 400, err.Error(), err)
	}

	return http.Render(c, 200, ord)
}

func route(router *zip.Group, prefix string) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	api := router.Group(prefix)
	api.Use(middleware.AccessControl("*"))

	// Hosted checkout sessions. AUTHENTICATED like every sibling: a valid
	// service token / per-org Published storefront token (or IAM principal) is
	// required BEFORE any org resolution or Square call. There is no anonymous
	// path to mint a payment link — the org is taken from the validated token,
	// never the request body.
	if prefix == "/checkout" {
		api.Raw(nethttp.MethodPost, "/sessions", publishedRequired, Sessions)
	}

	// Auth and Capture Flow (Two-step Payment)
	api.Raw(nethttp.MethodPost, "/authorize", publishedRequired, Authorize)
	api.Raw(nethttp.MethodPost, "/authorize/:orderid", publishedRequired, Authorize)
	api.Raw(nethttp.MethodPost, "/capture/:orderid", publishedRequired, Capture)

	// Charge Flow (implicit Auth+Capture)
	api.Raw(nethttp.MethodPost, "/charge", publishedRequired, Charge)

	// Confirm / Cancel Flow
	api.Raw(nethttp.MethodPost, "/confirm/:orderid", publishedRequired, Confirm)
	api.Raw(nethttp.MethodPost, "/cancel/:orderid", publishedRequired, Cancel)

	// Deprecated (should use normal authorization flow to initiate)
	api.Raw(nethttp.MethodPost, "/paypal", publishedRequired, Authorize)
	api.Raw(nethttp.MethodPost, "/paypal/pay", publishedRequired, Authorize)

	api.Raw(nethttp.MethodGet, "/ethereum/lookup/:proxyaddress", adminRequired, ethereum.Lookup)

	// Wire transfer endpoints
	api.Raw(nethttp.MethodGet, "/wire/instructions", wire.Instructions)
	api.Raw(nethttp.MethodPost, "/wire/credit", adminRequired, wire.Credit)
}

func Route(router *zip.Group, args ...zip.Handler) {
	route(router, "") // Deprecated
	route(router, "/checkout")
}
