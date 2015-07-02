package tasks

import (
	"time"

	"appengine"

	"appengine/memcache"

	"github.com/gin-gonic/gin"
	sg "github.com/stripe/stripe-go"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
	"crowdstart.com/util/task"
)

func cacheOrganization(ctx appengine.Context, org *organization.Organization) {
	nsctx := org.Namespace(ctx)

	item := &memcache.Item{
		Key:   "organization",
		Value: org.JSON(),
	}

	if err := memcache.Set(nsctx, item); err != nil {
		log.Error("Unable to cache organization: %v", err, ctx)
	}
}

// May be called one of two ways:
//   1. As an HTTP task from the generated pages, append organization=name to specify organization.
//	 2. As a delay Func, in which case organization should be specified as an extra argument.
var SyncCharges = task.Func("stripe-sync-charges", func(c *gin.Context, args ...interface{}) {
	db := datastore.New(c)
	org := organization.New(db)

	orgname := ""
	test := false

	// If we're called as an HTTP web task, we need to get organization off query string
	if c.Request.URL != nil {
		query := c.Request.URL.Query()
		orgname = query.Get("organization")
		test_ := query.Get("test")
		if test_ == "1" || test_ == "true" {
			test = true
		}
	}

	// If an extra argument is passed in, this is being called as a delay.Func,
	// get organization as extra parameter
	if len(args) == 1 {
		orgname = args[0].(string)
	}

	// Lookup organization
	if err := org.GetById(orgname); err != nil {
		log.Error("Unable to find organization(%s). %#v", orgname, err, c)
		return
	}

	// Pass Stripe access token to all subsequent requests
	token := org.StripeToken()

	ctx := db.Context

	// Create stripe client for this organization
	client := stripe.New(ctx, token)

	// Get all stripe charges
	params := &sg.ChargeListParams{}

	// Check for test flag
	if test {
		params.Filters.AddFilter("include[]", "", "total_count")
		params.Filters.AddFilter("limit", "", "10")
		params.Filters.AddFilter("starting_after", "", "ch_16FyN1F118aqM8IJCHWi6Mkx")
		params.Single = true
	}

	// Cache organization, namespace to use for later queries
	cacheOrganization(ctx, org)
	ns := org.Name

	// Get iterator for Stripe charges
	i := client.Charges.List(params)
	for i.Next() {
		// Get next charge
		ch := stripe.Charge(*i.Charge())

		// Update payment, using the namespaced context (i hope)
		start := time.Now()
		ChargeSync.Call(ctx, ns, token, ch, start)
	}

	if err := i.Err(); err != nil {
		log.Error("Error while iterating over charges. %#v", err, ctx)
	}
})
