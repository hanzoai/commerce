package tasks

import (
	"time"

	"github.com/gin-gonic/gin"
	sg "github.com/stripe/stripe-go/v75"

	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/middleware"
	"hanzo.io/models/organization"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/util/task"
)

// May be called one of two ways:
//  1. As an HTTP task from the generated pages, append organization=name to specify organization.
//  2. As a delay Func, in which case organization should be specified as an extra argument.
var SyncCharges = task.Func("stripe-sync-charges", func(c *gin.Context, args ...interface{}) {
	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)
	org := organization.New(db)

	orgname := ""
	test := false

	// If we're called as an HTTP web task, we need to get organization off query string
	if c.Request != nil {
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
		log.Error("Unable to find organization '%s': %v", orgname, err, c)
		return
	}

	// Get appropriate Stripe token for requests
	token := org.Stripe.Live.AccessToken
	if test {
		token = org.Stripe.Test.AccessToken
	}

	// Get new Stripe client
	ns := org.Name
	client := stripe.New(ctx, token)

	// Setup charge params
	params := &sg.ChargeListParams{}
	if test {
		params.Filters.AddFilter("include[]", "", "total_count")
		params.Filters.AddFilter("limit", "", "10")
		params.Filters.AddFilter("starting_after", "", "ch_16FyN1F118aqM8IJCHWi6Mkx")
		params.Single = true
	}

	// Get iterator for Stripe charges
	i := client.Charges.List(params)
	for i.Next() {
		// Get next charge
		ch := stripe.Charge(*i.Charge())

		// Update payment, order based on current charge
		start := time.Now()
		ChargeSync.Call(ctx, ns, token, ch, start)
	}

	if err := i.Err(); err != nil {
		log.Error("Error while iterating over charges. %#v", err, ctx)
	}
})
