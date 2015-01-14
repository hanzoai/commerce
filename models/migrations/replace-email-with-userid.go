package migrations

import (
	"time"

	"appengine"
	"appengine/delay"

	"crowdstart.io/datastore"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"

	. "appengine/datastore"

	. "crowdstart.io/models"
)

type OldContribution struct {
	Id            string
	Email         string
	FundingDate   string
	PaymentMethod string
	Perk          Perk
	Status        string
}

type OldToken struct {
	Id      string
	Email   string
	Used    bool
	Expired bool
}

type OldOrder struct {
	FieldMapMixin
	// Account         PaymentAccount
	BillingAddress  Address
	ShippingAddress Address
	CreatedAt       time.Time `schema:"-"`
	UpdatedAt       time.Time `schema:"-"`
	Id              string
	Email           string

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

var replaceEmailWithUserId = delay.Func("migrate-replace-email-with-userid", func(c appengine.Context) {
	db := datastore.New(c)
	q := queries.New(c)

	log.Debug("Migrating users")

	t := db.Query("user").Run(c)

	for {
		var u User
		k, err := t.Next(&u)

		if err != nil {
			// Done
			if err == Done {
				break
			}

			// Ignore field mismatch, otherwise skip record
			if _, ok := err.(*ErrFieldMismatch); !ok {
				log.Error("Error fetching user: %v", err, c)
				continue
			}
		}

		// Delete old User record
		log.Info("Deleting Key %v", k)
		db.Delete(k.Encode())

		// Empty the ID so Upsert auto generates it
		u.Id = ""
		q.UpsertUser(&u)
		log.Info("Upserting Encoded Key %v", u.Id)
	}

	log.Debug("Migrating contributions")

	t = db.Query("contribution").Run(c)

	for {
		var oCon OldContribution
		k, err := t.Next(&oCon)

		if err != nil {
			//Done
			if err == Done {
				break
			}

			// Error, ignore field mismatch
			if _, ok := err.(*ErrFieldMismatch); ok {
				log.Error("Contribution appears to be Updated: %v", err, c)
				continue
			}
		}

		// Get the corresponding user
		var u User
		if err = q.GetUserByEmail(oCon.Email, &u); err != nil {
			log.Error("Could not look up user: %v\n%v", oCon.Email, err)
			continue
		}

		// Update to new record and replace old one
		con := Contribution{
			Id:            oCon.Id,
			UserId:        u.Id,
			FundingDate:   oCon.FundingDate,
			PaymentMethod: oCon.PaymentMethod,
			Perk:          oCon.Perk,
			Status:        oCon.Status,
		}

		db.PutKey("contribution", k, &con)
	}

	log.Debug("Migrating tokens")

	t = db.Query("token").Run(c)

	for {
		var oTo OldToken
		k, err := t.Next(&oTo)

		if err != nil {
			//Done
			if err == Done {
				break
			}

			// Error, ignore field mismatch
			if _, ok := err.(*ErrFieldMismatch); ok {
				log.Error("Token appears to be Updated: %v", err, c)
				continue
			}
		}

		// Get the corresponding user
		var u User
		if err = q.GetUserByEmail(oTo.Email, &u); err != nil {
			log.Error("Could not look up user: %v\n%v", oTo.Email, err)
			break
		}

		// Update to new record and replace old one
		to := Token{
			Id:      oTo.Id,
			UserId:  u.Id,
			Used:    oTo.Used,
			Expired: oTo.Expired,
		}

		db.PutKey("token", k, &to)
	}

	log.Debug("Migrating orders")

	t = db.Query("order").Run(c)

	for {
		var oO OldOrder
		k, err := t.Next(&oO)

		if err != nil {
			//Done
			if err == Done {
				break
			}

			// Error, ignore field mismatch
			if _, ok := err.(*ErrFieldMismatch); ok {
				log.Error("Order appears to be Updated: %v", err, c)
				continue
			}
		}

		// Get the corresponding user
		var u User
		if err = q.GetUserByEmail(oO.Email, &u); err != nil {
			log.Error("Could not look up user: %v\n%v", oO.Email, err)
			break
		}

		// Update to new record and replace old one
		o := Order{
			BillingAddress:  oO.BillingAddress,
			ShippingAddress: oO.ShippingAddress,
			CreatedAt:       oO.CreatedAt,
			UpdatedAt:       oO.UpdatedAt,
			Id:              oO.Id,
			UserId:          u.Id,
			Shipping:        oO.Shipping,
			Tax:             oO.Tax,
			Subtotal:        oO.Subtotal,
			Total:           oO.Total,
			Items:           oO.Items,
			StripeTokens:    oO.StripeTokens,
			Charges:         oO.Charges,
			CampaignId:      oO.CampaignId,
			Preorder:        oO.Preorder,
			Cancelled:       oO.Cancelled,
			Shipped:         oO.Shipped,
			Test:            oO.Test,
		}

		db.PutKey("order", k, &o)
	}
})
