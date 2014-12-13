package fixtures

import (
	"encoding/csv"
	"os"
	"strings"

	"appengine"
	"appengine/delay"

	"crowdstart.io/datastore"
	"crowdstart.io/util/log"

	. "crowdstart.io/models"
)

var international = delay.Func("fixtures-international", func(c appengine.Context) {
	log.Debug("Installing international fixtures...")
	db := datastore.New(c)

	csvfile, err := os.Open("resources/contributions-old-international-perk.csv")
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

		perkId := row[1]
		pledgeId := row[2]

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
	}
})
