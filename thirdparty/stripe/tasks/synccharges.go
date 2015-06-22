package tasks

import (
	"time"

	"github.com/gin-gonic/gin"
	sg "github.com/stripe/stripe-go"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
	"crowdstart.com/util/task"
)

var SyncCharges = task.Func("stripe-sync-charges", func(c *gin.Context) {
	db := datastore.New(c)
	org := organization.New(db)
	ctx := org.Context()

	// Get organization off query
	query := c.Request.URL.Query()
	orgname := query.Get("organization")
	test := query.Get("test")

	// Lookup organization
	if err := org.GetById(orgname); err != nil {
		log.Error("Unable to find organization(%s). %#v", orgname, err, c)
		return
	}

	// Create stripe client for this organization
	client := stripe.New(ctx, org.StripeToken())

	// Get all stripe charges
	params := &sg.ChargeListParams{}
	if test == "1" || test == "true" {
		params.Filters.AddFilter("include[]", "", "total_count")
		params.Filters.AddFilter("limit", "", "10")
		params.Filters.AddFilter("starting_after", "", "ch_16FyN1F118aqM8IJCHWi6Mkx")
		params.Single = true
	}

	// Get namespace to use for later queries
	ns := org.Name

	i := client.Charges.List(params)
	for i.Next() {
		// Get next charge
		ch := stripe.Charge(*i.Charge())

		// Update payment, using the namespaced context (i hope)
		start := time.Now()
		UpdatePayment.Call(ctx, ns, ch, start)
	}

	if err := i.Err(); err != nil {
		log.Error("Error while iterating over charges. %#v", err, ctx)
	}
})
