package api

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/thirdparty/stripe"
	"github.com/hanzoai/commerce/thirdparty/stripe/tasks"
	"github.com/hanzoai/commerce/delay"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

// Decode Stripe payload
func decodeEvent(c *gin.Context) (*stripe.Event, error) {
	event := new(stripe.Event)
	if err := json.Decode(c.Request.Body, event); err != nil {
		log.Error("Could not Decode:\n%s", c.Request.Body, c)
		return nil, fmt.Errorf("Failed to parse Stripe webhook: %v", err)
	}

	log.JSON("Received '%s'", event.Type, event)
	return event, nil
}

// Get organization and token for event
func getToken(ctx context.Context, event *stripe.Event) (*organization.Organization, string, error) {
	db := datastore.New(ctx)
	org := organization.New(db)

	// Try to find organization with connected Stripe account
	// TODO: Make it impossible to connect the same user to multiple organizations
	ok, err := org.Query().Filter("Stripe.UserId=", event.Account).Get()
	if err != nil {
		return nil, "", fmt.Errorf("Failed to query organization associated with Stripe account '%s': %v\n%#v", event.Account, err, event, ctx)
	}

	if !ok {
		return nil, "", fmt.Errorf("No organization associated with Stripe account '%s'\n%#v", event.Account, event, ctx)
	}

	// Look up access token (if we don't have this we won't bother processing event)
	token, err := org.GetStripeAccessToken(event.Account)
	if err != nil {
		return nil, "", fmt.Errorf("No access token found for organization '%s', with matching Stripe user '%s': %v", org.Name, event.Account, err)
	}

	return org, token, nil
}

// Unmarshal raw stripe event object
func unmarshal(ctx context.Context, event *stripe.Event, dst interface{}) interface{} {
	if err := json.Unmarshal(event.Data.Raw, dst); err != nil {
		log.Error("Failed to unmarshal stripe event %v: %#v", err, event, ctx)
		return nil
	}
	return dst
}

// Add task to taskqueue to process this event
func addTask(fn *delay.Function, ctx context.Context, event *stripe.Event, org *organization.Organization, token string, obj interface{}) {
	val := reflect.ValueOf(obj).Elem().Interface()
	args := []interface{}{org.Name, token, val, time.Now()}
	if err := fn.Call(ctx, args...); err != nil {
		log.Error("Failed to create task to process Stripe webhook event '%s': %v\n%#v", event.Type, err, event, ctx)
	}
}

// Handle stripe webhook POSTs
func Webhook(c *gin.Context) {
	// Decode webhook event
	event, err := decodeEvent(c)
	if err != nil {
		http.Fail(c, 500, err.Error(), err)
		return
	}

	// Ignore test webhooks
	if !event.Live {
		// TODO: Support this?
		c.String(200, "ok")
		return
	}

	// Get App Engine context
	ctx := middleware.GetAppEngine(c)

	// Ensure event is associated with a connected account with a valid stripe token
	org, token, err := getToken(ctx, event)
	if err != nil {
		log.Error(err, ctx)
		// Act like everything was cool
		c.String(200, "ok")
		return
	}

	// Hydrate credentials from KMS (overrides datastore-stored tokens)
	if v, ok := c.Get("kms"); ok {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, ctx)
			}
			// Re-resolve token after hydration
			if t, err := org.GetStripeAccessToken(event.Account); err == nil && t != "" {
				token = t
			}
		}
	}

	// Process event accordingly
	switch event.Type {
	case "charge.succeeded":
		if ch := unmarshal(ctx, event, &stripe.Charge{}); ch != nil {
			addTask(tasks.FeeSync, ctx, event, org, token, ch)
		}
	case "charge.captured", "charge.failed", "charge.refunded", "charge.updated":
		if ch := unmarshal(ctx, event, &stripe.Charge{}); ch != nil {
			addTask(tasks.ChargeSync, ctx, event, org, token, ch)
		}
	case "charge.dispute.closed", "charge.dispute.created", "charge.dispute.funds_reinstated", "charge.dispute.funds_withdrawn", "charge.dispute.updated":
		if dis := unmarshal(ctx, event, &stripe.Dispute{}); dis != nil {
			addTask(tasks.DisputeSync, ctx, event, org, token, dis)
		}
	case "transfer.created", "transfer.failed", "transfer.paid", "transfer.reversed", "transfer.updated":
		if tr := unmarshal(ctx, event, &stripe.Transfer{}); tr != nil {
			addTask(tasks.TransferSync, ctx, event, org, token, tr)
		}
	case "source.chargeable":
		// bitcoin payment just became chargable
	case "source.canceled":
		// bitcoin payment just expired
	case "ping":
		c.String(200, "pong")
		return

	default:
		log.Warn("Unsupported Stripe event '%s': %#v", event.Type, event, c)
		return
	}

	// All good
	c.String(200, "ok")
}
