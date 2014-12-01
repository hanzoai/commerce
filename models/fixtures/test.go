package fixtures

import (
	"appengine"
	"appengine/delay"
	"encoding/csv"
	"os"
	"strings"

	"crowdstart.io/datastore"
	"crowdstart.io/util/log"

	. "crowdstart.io/models"
)

var Test = delay.Func("install-test-fixtures", func(c appengine.Context) {
	log.Debug("Installing test fixtures...")
	db := datastore.New(c)

	csvfile, err := os.Open("resources/contributions-test.csv")
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
	//	                                    Shipping Country         17
	for i := 0; true; i++ {
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

		perkId := row[1]
		pledgeId := row[2]

		// Create token
		token := new(InviteToken)
		token.Id = row[0]
		token.Email = email
		db.PutKey("invite-token", token.Id, token)

		// Save contribution
		contribution := Contribution{
			Id:            pledgeId,
			Perk:          perks[perkId],
			Status:        row[3],
			FundingDate:   row[4],
			PaymentMethod: row[5],
			Email:         email,
		}
		db.PutKey("contribution", pledgeId, &contribution)

		// Create user
		user := new(User)
		user.Id = email
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

		db.PutKey("user", user.Email, user)

		log.Debug("User %#v", user)
		log.Debug("InviteToken: %#v", token)
	}
})
