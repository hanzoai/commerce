package migrations

import (
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/delay"

	. "crowdstart.io/models"
	"crowdstart.io/util/log"
)

type BrokenOrder struct {
	FieldMapMixin
	// Account         PaymentAccount
	BillingAddress  Address
	ShippingAddress Address
	CreatedAt       time.Time `schema:"-"`
	UpdatedAt       time.Time `schema:"-"`
	Id              string
	Email           string
	UserId          string

	// TODO: Recalculate Shipping/Tax on server
	Shipping int64
	Tax      int64
	Subtotal int64 `schema:"-"`
	Total    int64 `schema:"-"`

	Items []LineItem

	// Slices in order to record failed tokens/charges
	StripeTokens []string `schema:"-"`
	Charges      []Charge `schema:"-"`

	// Need to save campaign id
	CampaignId string

	Preorder  bool
	Cancelled bool
	Shipped   bool
	// ShippingOption  ShippingOption

	Test bool
}

var listBrokenOrders = delay.Func(
	"list-broken-orders",
	newMigration(
		"list-broken-orders",
		"order",
		new(BrokenOrder),
		func(c appengine.Context, k *datastore.Key, object interface{}) error {
			switch o := object.(type) {
			case *BrokenOrder:
				// Get the corresponding user
				if o.Email != "" {
					log.Info("Recording Broken Order %v", o.Email, c)
					newK := datastore.NewKey(c, "broken-order", k.StringID(), k.IntID(), nil)
					if _, err := datastore.Put(c, newK, o); err != nil {
						log.Error("Could not Put Broken Order because %v", err, c)
						return err
					}
				}
			}

			return nil
		}))
