package indiegogo

import (
	"encoding/csv"
	"os"
	"strings"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/util/log"

	. "crowdstart.io/models"
)

func ImportCSV(db *datastore.Datastore, filename string) {
	csvfile, err := os.Open("resources/contributions.csv")
	defer csvfile.Close()
	if err != nil {
		log.Fatal("Failed to open CSV File: %v", err)
	}

	reader := csv.NewReader(csvfile)
	reader.FieldsPerRecord = -1

	// Skip header
	reader.Read()

	// CSV layout:
	// Token Id           0  Appearance 6   Shipping Name            11
	// Perk ID            1  Name       7   Shipping Address         12
	// Pledge ID          2  Email      8   Shipping Address 2       13
	// Fulfillment Status 3  Amount     9   Shipping City            14
	// Funding Date       4  Perk       10  Shipping State/Province  15
	// Payment Method     5                 Shipping Zip/Postal Code 16
	for i := 0; true; i++ {
		// Only import first 25 in development
		if config.IsDevelopment && i > 25 {
			break
		}

		// Loop until exhausted
		row, err := reader.Read()
		if err != nil {
			break
		}

		// Normalize various bits
		email := row[8]
		email = strings.ToLower(email)

		// Da fuq, Indiegogo?
		postalCode := row[16]
		postalCode = strings.Trim(postalCode, "=")
		postalCode = strings.Trim(postalCode, "\"")

		// Title case name
		name := strings.SplitN(row[7], " ", 2)
		firstName := ""
		lastName := ""

		if len(name) > 0 {
			firstName = strings.Title(strings.ToLower(name[0]))
		}
		if len(name) > 1 {
			lastName = strings.Title(strings.ToLower(name[1]))
		}

		city := strings.Title(strings.ToLower(row[14]))

		tokenId := row[0]
		perkId := row[1]
		pledgeId := row[2]

		// Create user
		user := new(User)
		user.Email = email
		user.FirstName = firstName
		user.LastName = lastName

		address := Address{
			Line1:      row[12],
			Line2:      row[13],
			City:       city,
			State:      row[15],
			PostalCode: postalCode,
			Country:    row[17],
		}

		user.ShippingAddress = address
		user.BillingAddress = address

		// No longer updating user information in production, as it would clobber any customized information.
		if !config.IsProduction {
			user.Upsert(db)
		}

		// Create token
		token := new(Token)
		token.Id = tokenId
		token.UserId = user.Id
		token.Email = user.Email

		db.PutKey("invite-token", tokenId, token)

		// Save contribution
		contribution := Contribution{
			Id:            pledgeId,
			Perk:          Perks[perkId],
			Status:        row[3],
			FundingDate:   row[4],
			PaymentMethod: row[5],
			UserId:        user.Id,
		}
		db.PutKey("contribution", pledgeId, &contribution)

		log.Debug("User: %#v", user)
		log.Debug("Token: %#v", token)
	}
}
