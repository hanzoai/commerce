package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
)

// product_events.go publishes catalog lifecycle events from the product REST path —
// the reverse half of the storefront loop (the content lane consumes product.created
// to auto-render the product's ecom asset, keyed by design == slug).
//
// It mirrors the order publishers (checkout/authorize.go): read the process Publisher
// off the request context, fire the event as a fire-and-forget goroutine, and be a total
// no-op when no publisher is wired (NATS absent) — never blocking or failing the write.
// It is route middleware rather than a custom handler so the generic REST create/update
// stays the ONE write path; the middleware only observes its outcome.

// publishProductEvents is product-route middleware: after a successful create/replace it
// fires the matching commerce.product.* event. The body is read BEFORE the handler runs
// (c.Body() is the reusable request buffer, so the handler still sees the same bytes);
// the event is sent only when the write actually succeeded (2xx). Fail-safe throughout:
// a missing publisher, an unresolved org, a body without a slug, or a non-2xx is a silent
// no-op.
func publishProductEvents(c *zip.Ctx) error {
	subject := writeSubject(c)
	var body productBody
	if subject != "" {
		body = peekProductBody(c) // c.Body() stays available to the downstream handler
	}

	if err := c.Next(); err != nil {
		return err
	}

	if subject == "" || body.Slug == "" {
		return nil
	}
	if st := c.Fiber().Response().StatusCode(); st < 200 || st >= 300 {
		return nil // the create/update did not commit — publish nothing
	}
	pub := c.Locals("publisher")
	if pub == nil {
		return nil
	}
	p, ok := pub.(*events.Publisher)
	if !ok || p == nil {
		return nil
	}
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return nil
	}
	orgName := org.Name
	productID := eventProductID(c)
	go func() {
		ctx := context.Background()
		var err error
		if subject == events.SubjectProductCreated {
			err = p.PublishProductCreated(ctx, orgName, productID, body.Slug, body.Name, body.SKU)
		} else {
			err = p.PublishProductUpdated(ctx, orgName, productID, body.Slug, body.Name, body.SKU)
		}
		if err != nil {
			// The request ctx is pooled/released once this middleware returns, so
			// the fire-and-forget goroutine must not reference c for log enrichment.
			log.Error("publish catalog event %q: %v", subject, err)
		}
	}()
	return nil
}

// writeSubject classifies the request into the catalog event it should emit, or "" when
// it is not a create/replace. POST to the collection is a create; PUT/PATCH to an item
// is a replace/patch. POST to an item (the legacy method-override) is deliberately NOT
// classified — its effective verb (patch/update/delete) is ambiguous here, so it is left
// to callers using the explicit verbs.
func writeSubject(c *zip.Ctx) string {
	switch c.Method() {
	case http.MethodPost:
		if strings.TrimSpace(c.Param("productid")) == "" {
			return events.SubjectProductCreated
		}
		return ""
	case http.MethodPut, http.MethodPatch:
		return events.SubjectProductUpdated
	default:
		return ""
	}
}

// productBody is the minimal slice of a product write the event carries: the slug is the
// join key content renders against (design == slug); name/sku are context.
type productBody struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	SKU  string `json:"sku"`
}

// peekProductBody decodes the product write from the request buffer. c.Body() is
// fiber's retained request body, so reading it here does NOT consume it — the downstream
// create/update handler decodes the same bytes. A decode error yields the zero body (no
// event), never a failed request.
func peekProductBody(c *zip.Ctx) productBody {
	var b productBody
	_ = json.Unmarshal(c.Body(), &b)
	b.Slug = strings.TrimSpace(b.Slug)
	return b
}

// eventProductID resolves the product id for the event: the item param on an update, or
// the id the create handler wrote into the Location header (path/<id>). Best-effort — the
// consumer joins on slug, so an empty id is harmless.
func eventProductID(c *zip.Ctx) string {
	if id := strings.TrimSpace(c.Param("productid")); id != "" {
		return id
	}
	loc := c.Fiber().GetRespHeader("Location")
	if i := strings.LastIndexByte(loc, '/'); i >= 0 {
		return strings.TrimSpace(loc[i+1:])
	}
	return ""
}
