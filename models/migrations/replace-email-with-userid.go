package migrations

import (
	"errors"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/delay"

	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"

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

var replaceEmailWithUserIdForUser = delay.Func(
	"migrate-replace-email-with-userid-for-user",
	newMigration(
		"migration-replace-email-with-userid-for-user",
		"user",
		new(User),
		func(c appengine.Context, k *datastore.Key, object interface{}) error {
			switch u := object.(type) {
			case *User:
				if u.Id != u.Email && u.Id != "" {
					log.Info("Do not need to Migrate Key %v", k, c)
					return nil
				}

				// Empty the ID so Upsert auto generates it
				id, _, err := datastore.AllocateIDs(c, "user", nil, 1)
				if err != nil {
					log.Error("Could not allocate Key  because %v", err, c)
					return err
				}

				newK := datastore.NewKey(c, "user", "", id, nil)
				u.Id = newK.Encode()

				log.Info("Inserting Key %v", newK, c)

				if _, err := datastore.Put(c, newK, u); err != nil {
					log.Error("Could not Put User because %v", err, c)
					return err
				}

				// Delete old User record
				log.Info("Deleting Key %v", k, c)
				if err = datastore.Delete(c, k); err != nil {
					log.Error("Could not Delete User %v because %v", k, err, c)
				}

				return err
			}

			return errors.New("Invalid type, required: *User")
		}))

var replaceEmailWithUserIdForContribution = delay.Func(
	"migrate-replace-email-with-userid-for-contribution",
	newMigration(
		"migration-replace-email-with-userid-for-contribution",
		"contribution",
		new(OldContribution),
		func(c appengine.Context, k *datastore.Key, object interface{}) error {
			switch oCon := object.(type) {
			case *OldContribution:
				// Get the corresponding user
				q := queries.New(c)

				var u User
				if err := q.GetUserByEmail(oCon.Email, &u); err != nil {
					log.Warn("Could not look up user: %v\n%v", oCon.Email, err, c)
					return nil
				}

				// 	// Update to new record and replace old one
				con := Contribution{
					Id:            oCon.Id,
					UserId:        u.Id,
					FundingDate:   oCon.FundingDate,
					PaymentMethod: oCon.PaymentMethod,
					Perk:          oCon.Perk,
					Status:        oCon.Status,
				}

				log.Info("Upserting Key %v", k, c)
				datastore.Put(c, k, &con)
			}

			return errors.New("Invalid type, required: *OldContribution")
		}))

var replaceEmailWithUserIdForToken = delay.Func(
	"migrate-replace-email-with-userid-for-token",
	newMigration(
		"migration-replace-email-with-userid-for-token",
		"token",
		new(OldToken),
		func(c appengine.Context, k *datastore.Key, object interface{}) error {
			switch oTo := object.(type) {
			case *OldToken:
				// Get the corresponding user
				q := queries.New(c)

				var u User
				if err := q.GetUserByEmail(oTo.Email, &u); err != nil {
					log.Warn("Could not look up user: %v\n%v", oTo.Email, err, c)
					return nil
				}

				// 	// Update to new record and replace old one
				to := Token{
					Id:      oTo.Id,
					UserId:  u.Id,
					Used:    oTo.Used,
					Expired: oTo.Expired,
				}

				log.Info("Upserting Key %v", k, c)
				datastore.Put(c, k, &to)
			}

			return errors.New("Invalid type, required: *OldToken")
		}))

var replaceEmailWithUserIdForOrder = delay.Func(
	"migrate-replace-email-with-userid-for-order",
	newMigration(
		"migration-replace-email-with-userid-for-order",
		"order",
		new(OldOrder),
		func(c appengine.Context, k *datastore.Key, object interface{}) error {
			switch oO := object.(type) {
			case *OldOrder:
				// Get the corresponding user
				q := queries.New(c)

				var u User
				if err := q.GetUserByEmail(oO.Email, &u); err != nil {
					log.Warn("Could not look up user: %v\n%v", oO.Email, err, c)
					return nil
				}

				// 	// Update to new record and replace old one
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

				log.Info("Upserting Key %v", k, c)
				datastore.Put(c, k, &o)
			}

			return errors.New("Invalid type, required: *OldOrder")
		}))
