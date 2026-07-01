package checkout

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/api/checkout/ethereum"
	"github.com/hanzoai/commerce/api/checkout/wire"
	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/router"
)

var orderEndpoint = config.UrlFor("api", "/order/")

func getOrganizationAndOrder(c *gin.Context) (*organization.Organization, *order.Order, error) {
	// Get organization for this user
	org := middleware.GetOrganization(c)

	// Hydrate payment credentials from KMS
	if v, ok := c.Get("kms"); ok {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
			}
		}
	}

	// Set up the db with the namespaced context
	ctx := org.Namespaced(c)
	db := datastore.New(ctx)

	// Create order that's properly namespaced
	ord := order.New(db)

	// Get order if an existing order was referenced
	if id := c.Params.ByName("orderid"); id != "" {
		if err := ord.GetById(id); err != nil {
			http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
			return nil, nil, OrderDoesNotExist
		}
	}

	return org, ord, nil
}

func Authorize(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	if _, err = authorize(c, org, ord); err != nil {
		log.Error("Error %v %v", err.Error(), err, c)
		http.Fail(c, 400, err.Error(), err)
		return
	}

	log.JSON(ord)
	c.Writer.Header().Add("Location", orderEndpoint+ord.Id())
	log.JSON(ord)
	http.Render(c, 200, ord)
}

func Capture(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	if err = capture(c, org, ord); err != nil {
		log.Error("Error during capture %v", err, c)
		http.Fail(c, 400, "Error during capture", err)
		return
	}

	c.Writer.Header().Add("Location", orderEndpoint+ord.Id())
	http.Render(c, 200, ord)
}

func Charge(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	// Do authorization
	if _, err = authorize(c, org, ord); err != nil {
		log.Error("Error %v %v", err.Error(), err, c)
		http.Fail(c, 400, "Error during authorize", err)
		return
	}

	// Do capture using order from authorization
	if err = capture(c, org, ord); err != nil {
		log.Error("Error during capture %v", err, c)
		http.Fail(c, 400, "Error during capture", err)
		return
	}

	c.Writer.Header().Add("Location", orderEndpoint+ord.Id())
	http.Render(c, 200, ord)
}

func Refund(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	// Idempotency guard (money-critical). A refund is a non-idempotent money
	// move: two identical POSTs (a client retry, a double-click, a proxy replay)
	// would otherwise each hit the gateway and double-refund. When the caller
	// supplies an idempotency key, a replay returns the FIRST result instead of
	// refunding again. The guard is scoped per order so keys never cross orders.
	//
	// Scope of protection: this de-dups OUR ledger + replays OUR response. For
	// the narrow concurrent-first-submit window (two brand-new identical
	// requests racing before either records the guard) the gateway is the final
	// backstop — Square/Stripe both honor an idempotency key on the refund call.
	// square.Refund already guards over-refund (Refunded+amt > Total ⇒ reject);
	// the guard here adds retry/replay safety on top.
	idemKey := strings.TrimSpace(c.GetHeader("X-Idempotency-Key"))
	db := datastore.New(org.Namespaced(c))
	if idemKey != "" {
		scope := "refund:" + ord.Id()
		rec, replay, gerr := idempotencykey.Begin(db, scope, idemKey)
		if gerr != nil {
			http.Fail(c, 500, "idempotency guard failed", gerr)
			return
		}
		if replay {
			// A prior request with this key already ran. Return its stored
			// outcome verbatim; do NOT refund again.
			if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
				c.Data(200, "application/json", []byte(rec.Response))
				return
			}
			// In-flight (started but not completed): a concurrent request owns
			// this key. Fail closed with 409 so the caller retries, rather than
			// risk a parallel double refund.
			http.Fail(c, 409, "a refund with this idempotency key is already in progress", errRefundInFlight)
			return
		}

		if err := refund(c, org, ord); err != nil {
			http.Fail(c, 400, err.Error(), err)
			return
		}
		// Record the successful outcome for future replays.
		if resp, merr := json.Marshal(ord); merr == nil {
			_ = idempotencykey.Complete(rec, string(resp))
		}
		http.Render(c, 200, ord)
		return
	}

	// No idempotency key supplied — legacy behavior (over-refund guard still
	// applies in square.Refund). Callers that move money SHOULD send a key.
	if err := refund(c, org, ord); err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	http.Render(c, 200, ord)
}

func Cancel(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	if err := cancel(c, org, ord); err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	http.Render(c, 200, ord)
}

func Confirm(c *gin.Context) {
	org, ord, err := getOrganizationAndOrder(c)
	if err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	if err := confirm(c, org, ord); err != nil {
		http.Fail(c, 400, err.Error(), err)
		return
	}

	http.Render(c, 200, ord)
}

func route(router router.Router, prefix string) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	api := router.Group(prefix)
	api.Use(middleware.AccessControl("*"))

	// Hosted checkout sessions
	if prefix == "/checkout" {
		api.POST("/sessions", Sessions)
	}

	// Auth and Capture Flow (Two-step Payment)
	api.POST("/authorize", publishedRequired, Authorize)
	api.POST("/authorize/:orderid", publishedRequired, Authorize)
	api.POST("/capture/:orderid", publishedRequired, Capture)

	// Charge Flow (implicit Auth+Capture)
	api.POST("/charge", publishedRequired, Charge)

	// Confirm / Cancel Flow
	api.POST("/confirm/:orderid", publishedRequired, Confirm)
	api.POST("/cancel/:orderid", publishedRequired, Cancel)

	// Deprecated (should use normal authorization flow to initiate)
	api.POST("/paypal", publishedRequired, Authorize)
	api.POST("/paypal/pay", publishedRequired, Authorize)

	api.GET("/ethereum/lookup/:proxyaddress", adminRequired, ethereum.Lookup)

	// Wire transfer endpoints
	api.GET("/wire/instructions", wire.Instructions)
	api.POST("/wire/credit", adminRequired, wire.Credit)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	route(router, "") // Deprecated
	route(router, "/checkout")
}
