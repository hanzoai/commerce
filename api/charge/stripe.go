package charge

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/campaign"
	"crowdstart.io/models2/order"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/user"
	"crowdstart.io/thirdparty/stripe"
	"crowdstart.io/util/json"
)

type ModelRef struct {
	Id string
}

// Post to this the stripe card params
//	{
//		card: {
//			number: "4242424242424242",
//			month:  "12",
//			year:   "2016",
//			cvc:    "123",
//		},
//		order: {
//			id: "1234",
//		}
//	}

type CardUser struct {
	Card  stripe.Card
	Order ModelRef
}

func ChargeStripe(c *gin.Context) {
	d := datastore.New(c)

	var cu CardUser

	json.Decode(c.Request.Body, &cu)

	card := cu.Card
	id := cu.Order.Id
	var o order.Order

	if err := d.Get(id, &o); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[Api.Charge.Stripe] %v", err)
		c.JSON(500, gin.H{"status": "unable to find order"})
		return
	}

	var ca campaign.Campaign
	if err := d.Get(o.CampaignId, &ca); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[Api.Charge.Stripe] %v", err)
		c.JSON(500, gin.H{"status": "unable to find order's campaign"})
		return
	}

	var org organization.Organization
	if err := d.Get(ca.OrganizationId, &org); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[Api.Charge.Stripe] %v", err)
		c.JSON(500, gin.H{"status": "unable to find order's organization"})
		return
	}

	var u user.User
	if err := d.Get(o.UserId, &u); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("[Api.Charge.Stripe] %v", err)
		c.JSON(500, gin.H{"status": "unable to find order's user"})
		return
	}

	stripe.NewToken(&card, config.Stripe.APIKey)
}
