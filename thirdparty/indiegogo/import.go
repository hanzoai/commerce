package indiegogo

import (
	"crowdstart.io/config"
	"crowdstart.io/datastore"

	. "crowdstart.io/models"
)

func CSVImport(db *datastore.Datastore, filename string) {
	for row := range CSVIterator(filename) {
		// Create user
		user := &User{
			Email:           row.Email,
			FirstName:       row.FirstName,
			LastName:        row.LastName,
			ShippingAddress: row.ShippingAddress,
			BillingAddress:  row.ShippingAddress,
		}

		// No longer updating user information in production, as it would clobber any customized information.
		if !config.IsProduction {
			user.Upsert(db)
		}

		// Create token
		token := &Token{
			Id:     row.TokenID,
			UserId: user.Id,
			Email:  user.Email,
		}

		db.PutKind("invite-token", token.Id, token)

		// Save contribution
		contribution := &Contribution{
			Id:            row.PledgeID,
			Perk:          Perks[row.PerkID],
			Status:        row.FulfillmentStatus,
			FundingDate:   row.FundingDate,
			PaymentMethod: row.PaymentMethod,
			UserId:        user.Id,
		}
		db.PutKind("contribution", row.PledgeID, contribution)
	}
}
