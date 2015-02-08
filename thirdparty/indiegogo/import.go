package indiegogo

import (
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/util/csv"

	. "crowdstart.io/models"
)

func ImportCSV(db *datastore.Datastore, filename string) {
	for record := range csv.Iterator(filename) {
		if config.IsDevelopment && record.Index > 25 {
			break // Only import first 25 in development
		}

		// Parse Row
		r := NewRow(record.Row)

		// Create user
		user := &User{
			Email:           r.Email,
			FirstName:       r.FirstName,
			LastName:        r.LastName,
			ShippingAddress: r.ShippingAddress,
			BillingAddress:  r.ShippingAddress,
		}

		// No longer updating user information in production, as it would clobber any customized information.
		if !config.IsProduction {
			user.Upsert(db)
		}

		// Create token
		token := &Token{
			Id:     r.TokenID,
			UserId: user.Id,
			Email:  user.Email,
		}

		db.PutKind("invite-token", token.Id, token)

		// Save contribution
		// contribution := &Contribution{
		// 	Id:            r.PledgeID,
		// 	Perk:          Perks[r.PerkID],
		// 	Status:        r.FulfillmentStatus,
		// 	FundingDate:   r.FundingDate,
		// 	PaymentMethod: r.PaymentMethod,
		// 	UserId:        user.Id,
		// }
		// db.PutKind("contribution", r.PledgeID, contribution)
	}
}
