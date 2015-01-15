package migrations

import (
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/delay"

	"crowdstart.io/util/log"

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

var replaceEmailWithUserIdUserOnly = delay.Func("migrate-replace-email-with-userid-user-only", func(c appengine.Context) {
	log.Info("Migrating Users", c)

	t := datastore.NewQuery("user").Run(c)

	// Keys to batch delete
	var u User
	var id int64
	var k, newK *datastore.Key
	var err error
	var ok bool

	for {
		// Report memory stats
		k, err = t.Next(&u)

		if err != nil {
			// Done
			if err == datastore.Done {
				break
			}

			// Ignore field mismatch, otherwise skip record
			if _, ok = err.(*datastore.ErrFieldMismatch); !ok {
				log.Error("Error fetching user: %v\n%v", k, err, c)
				continue
			}
		}

		// Skip if we've already performed this
		if u.Id != u.Email {
			log.Info("Skipping Migrated User %v", u.Id, c)
			continue
		}

		log.Info("Migrating Key %v", u.Id, c)
		datastore.RunInTransaction(c, func(tc appengine.Context) error {
			// Empty the ID so Upsert auto generates it
			id, _, err = datastore.AllocateIDs(tc, "user", nil, 1)
			if err != nil {
				log.Error("Could not get Key %v", err, tc)
				return err
			}

			newK = datastore.NewKey(tc, "user", "", id, nil)
			u.Id = newK.Encode()

			log.Info("Inserting Key", id, tc)

			if _, err = datastore.Put(tc, newK, &u); err != nil {
				log.Error("Could not Put User %v", newK, tc)
				return err
			}

			log.Info("Deleting Key %v", k.StringID(), tc)

			// Delete old User record
			log.Info("Deleting Key %v", k, tc)
			if err != datastore.Delete(tc, k) {
				log.Error("Could not Delete User %v", newK, tc)
			}

			return err
		}, nil)
	}
})

var replaceEmailWithUserId = delay.Func("migrate-replace-email-with-userid", func(c appengine.Context) {
	// db := datastore.New(c)
	// q := queries.New(c)

	// log.Info("Migrating users", c)

	// t := db.Query("user").Run(c)

	// for {
	// 	var u User
	// 	k, err := t.Next(&u)

	// 	if err != nil {
	// 		// Done
	// 		if err == Done {
	// 			break
	// 		}

	// 		// Ignore field mismatch, otherwise skip record
	// 		if _, ok := err.(*ErrFieldMismatch); !ok {
	// 			log.Error("Error fetching user: %v\n%v", k, err, c)
	// 			continue
	// 		}
	// 	}

	// 	// Delete old User record
	// 	log.Info("Deleting Key %v", k, c)
	// 	db.Delete(k.Encode())

	// 	// Empty the ID so Upsert auto generates it

	// 	id := db.AllocateId("user")

	// 	u.Id = db.EncodeId("user", id)
	// 	newK, err := db.DecodeKey(u.Id)
	// 	if err != nil {
	// 		log.Error("Could not decode key: %v", newK, c)
	// 	}

	// 	db.PutKey("user", newK, &u)
	// 	log.Info("Inserting Encoded Key %v", u.Id, c)
	// }

	// log.Info("Migrating contributions", c)

	// t = db.Query("contribution").Run(c)

	// for {
	// 	var oCon OldContribution
	// 	k, err := t.Next(&oCon)

	// 	if err != nil {
	// 		//Done
	// 		if err == Done {
	// 			break
	// 		}

	// 		// Error, ignore field mismatch
	// 		if _, ok := err.(*ErrFieldMismatch); ok {
	// 			log.Error("Contribution appears to be Updated: %v", err, c)
	// 			continue
	// 		}
	// 	}

	// 	// Get the corresponding user
	// 	var u User
	// 	if err = q.GetUserByEmail(oCon.Email, &u); err != nil {
	// 		log.Error("Could not look up user: %v\n%v", oCon.Email, err, c)
	// 		continue
	// 	}

	// 	// Update to new record and replace old one
	// 	con := Contribution{
	// 		Id:            oCon.Id,
	// 		UserId:        u.Id,
	// 		FundingDate:   oCon.FundingDate,
	// 		PaymentMethod: oCon.PaymentMethod,
	// 		Perk:          oCon.Perk,
	// 		Status:        oCon.Status,
	// 	}

	// 	db.PutKey("contribution", k, &con)
	// }

	// log.Info("Migrating tokens", c)

	// t = db.Query("token").Run(c)

	// for {
	// 	var oTo OldToken
	// 	k, err := t.Next(&oTo)

	// 	if err != nil {
	// 		//Done
	// 		if err == Done {
	// 			break
	// 		}

	// 		// Error, ignore field mismatch
	// 		if _, ok := err.(*ErrFieldMismatch); ok {
	// 			log.Error("Token appears to be Updated: %v", err, c)
	// 			continue
	// 		}
	// 	}

	// 	// Get the corresponding user
	// 	var u User
	// 	if err = q.GetUserByEmail(oTo.Email, &u); err != nil {
	// 		log.Error("Could not look up user: %v\n%v", oTo.Email, err, c)
	// 		break
	// 	}

	// 	// Update to new record and replace old one
	// 	to := Token{
	// 		Id:      oTo.Id,
	// 		UserId:  u.Id,
	// 		Used:    oTo.Used,
	// 		Expired: oTo.Expired,
	// 	}

	// 	db.PutKey("token", k, &to)
	// }

	// log.Info("Migrating orders", c)

	// t = db.Query("order").Run(c)

	// for {
	// 	var oO OldOrder
	// 	k, err := t.Next(&oO)

	// 	if err != nil {
	// 		//Done
	// 		if err == Done {
	// 			break
	// 		}

	// 		// Error, ignore field mismatch
	// 		if _, ok := err.(*ErrFieldMismatch); ok {
	// 			log.Error("Order appears to be Updated: %v", err, c)
	// 			continue
	// 		}
	// 	}

	// 	// Get the corresponding user
	// 	var u User
	// 	if err = q.GetUserByEmail(oO.Email, &u); err != nil {
	// 		log.Error("Could not look up user: %v\n%v", oO.Email, err, c)
	// 		break
	// 	}

	// 	// Update to new record and replace old one
	// 	o := Order{
	// 		BillingAddress:  oO.BillingAddress,
	// 		ShippingAddress: oO.ShippingAddress,
	// 		CreatedAt:       oO.CreatedAt,
	// 		UpdatedAt:       oO.UpdatedAt,
	// 		Id:              oO.Id,
	// 		UserId:          u.Id,
	// 		Shipping:        oO.Shipping,
	// 		Tax:             oO.Tax,
	// 		Subtotal:        oO.Subtotal,
	// 		Total:           oO.Total,
	// 		Items:           oO.Items,
	// 		StripeTokens:    oO.StripeTokens,
	// 		Charges:         oO.Charges,
	// 		CampaignId:      oO.CampaignId,
	// 		Preorder:        oO.Preorder,
	// 		Cancelled:       oO.Cancelled,
	// 		Shipped:         oO.Shipped,
	// 		Test:            oO.Test,
	// 	}

	// 	db.PutKey("order", k, &o)
	// }
})
