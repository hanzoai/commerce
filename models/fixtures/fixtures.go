package fixtures

import (
	"encoding/csv"
	"os"
	"strings"

	"code.google.com/p/go.crypto/bcrypt"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/util/log"

	. "crowdstart.io/models"
)

var perks = map[string]Perk{
	"2123732": Perk{
		Id:                "2123732",
		Title:             "SKULLY AR-1",
		Description:       "Get 1 SKULLY AR-1 Motorcycle helmet at a limited introductory launch price. Free shipping to the United States. Sizes S-XXL in Matte Black or Gloss White. Team SKULLY will email you after the campaign for size and color choices.",
		Price:             "$1,399 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    1,
		GearQuantity:      1,
	},

	"2291929": Perk{
		Id:                "2291929",
		Title:             "SKULLY NATION GEAR",
		Description:       "Join Team SKULLY with a package of official gear: limited edition hat, shirt, and decals. Free shipping to the United States. Shirt sizes S-XXL (available in men’s and women’s cut), Flexfit hat sizes S/M, L/XL. Team SKULLY will email you after the campaign for cut and size choices. *Free w/ AR-1 preorder.",
		Price:             "$49 USD",
		EstimatedDelivery: "December 2014",
		HelmetQuantity:    0,
		GearQuantity:      1,
	},

	"2267279": Perk{
		Id:                "2267279",
		Title:             "$499 Now and $949 Due at Ship",
		Description:       "Reserve 1 SKULLY AR-1 Motorcycle helmet with $499 deposit, and $949* due when the AR-1 ships. Sizes S-XXL in Matte Black or Gloss White. Reserved helmets will ship after all regular pre-orders are shipped. No credit check necessary. Free shipping to the United States. *FOR INTERNATIONAL: additional shipping fee of $99.99 USD will apply to the second payment. SKULLY will email after the campaign for size and color choice, and will email you closer to your ship date to collect the second payment.",
		Price:             "$499 USD",
		EstimatedDelivery: "July 2015",
		HelmetQuantity:    1,
		GearQuantity:      1,
	},

	"2244337": Perk{
		Id:                "2244337",
		Title:             "International SKULLY AR-1",
		Description:       "Includes international shipping. Get 1 SKULLY AR-1 Motorcycle helmet at our international launch price. If you live outside of the United States, reserve your SKULLY AR-1 here. Sizes S-XXL in Matte Black or Gloss White. Team SKULLY will email you after the campaign for size and color choices.",
		Price:             "$1,499 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    1,
		GearQuantity:      1,
	},

	"2123523": Perk{
		Id:                "2123523",
		Title:             "Signature Edition SKULLY AR-1",
		Description:       "Get 1 Signature Edition SKULLY AR-1 Motorcycle helmet, hand-numbered and signed by CEO Marcus Weller. Sizes S-XXL in Matte Black or Gloss White. Team SKULLY will email you after the campaign for size and color choices.",
		Price:             "$1,999 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    1,
		GearQuantity:      1,
	},

	"2210249": Perk{
		Id:                "2210249",
		Title:             "Passenger 2-Pack Deal",
		Description:       "Get 2 SKULLY AR-1 Motorcycle helmets and save! Sizes S-XXL in Matte Black or Gloss White. Team SKULLY will email you after the campaign for size and color choices.",
		Price:             "$2,649 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    2,
		GearQuantity:      2,
	},

	"2291934": Perk{
		Id:                "2291934",
		Title:             "CLUB & DISTRIBUTOR 5 PACK",
		Description:       "Outfit your crew with 5 SKULLY AR-1s and save even more! Free shipping to the United States. Sizes S-XXL in Matte Black or Gloss White. Team SKULLY will email you after the campaign for size and color choices. *FOR INTERNATIONAL, add $375 USD for shipping.",
		Price:             "$6,495 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    5,
		GearQuantity:      5,
	},

	"2353031": Perk{
		Id:                "2353031",
		Title:             "20 AR-1 DISTRIBUTOR PACK",
		Description:       "Line your store shelves with 20 SKULLY AR-1s for massive savings! Free shipping to the United States. Sizes S-XXL in Matte Black or Gloss White. Team SKULLY will email you after the campaign for size and color choices. *FOR INTERNATIONAL, add $1499 USD for shipping.",
		Price:             "$24,979 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    20,
		GearQuantity:      20,
	},
}

func Install(db *datastore.Datastore) {
	log.Debug("Loading fixtures...")

	pwhash, _ := bcrypt.GenerateFromPassword([]byte("password"), 12)

	db.GetKey("user", "doesntexist", nil)

	// Default User (SKULLY)
	db.PutKey("user", "skully", &User{
		Id:           "skully",
		FirstName:    "Mitchell",
		LastName:     "Weller",
		Email:        "dev@hanzo.ai",
		Phone:        "(123) 456-7890",
		OrdersIds:    []string{},
		PasswordHash: pwhash,
	})

	// Default Campaign (SKULLY)
	db.PutKey("campaign", "skully", &Campaign{
		Id:    "skully",
		Title: "SKULLY AR-1",
	})

	// AR-1
	variants := []ProductVariant{
		ProductVariant{
			SKU:   "AR-1-BLACK-S",
			Size:  "S",
			Color: "Black",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-BLACK-M",
			Size:  "M",
			Color: "Black",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-BLACK-L",
			Size:  "L",
			Color: "Black",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-BLACK-XL",
			Size:  "XL",
			Color: "Black",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-BLACK-XXL",
			Size:  "XXL",
			Color: "Black",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-WHITE-S",
			Size:  "S",
			Color: "White",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-WHITE-M",
			Size:  "M",
			Color: "White",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-WHITE-L",
			Size:  "L",
			Color: "White",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-WHITE-XL",
			Size:  "XL",
			Color: "White",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-WHITE-XXL",
			Size:  "XXL",
			Color: "White",
			Price: 1499 * 100 * 100,
		},
	}

	for _, v := range variants {
		db.PutKey("variant", v.SKU, &v)
	}

	db.PutKey("product", "ar-1", &Product{
		Slug:        "ar-1",
		Title:       "SKULLY AR-1",
		Headline:    "The World's smartest helmet.",
		Excerpt:     "The World's smartest helmet, featuring a state-of-the-art head-up display, GPS and bluetooth.",
		Description: "",
		Variants:    variants,
		HeaderImage: Image{
			Alt: "SKULLY AR-1",
			Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544a2657e4b0ff95316b8ea0/1414359306658/",
			X:   1500,
			Y:   844,
		},
		Images: []Image{
			Image{
				Alt: "blackhelmet_store_1000px.jpg",
				Url: "/img/products/blackhelmet_store_1000px.jpg",
				X:   1000,
				Y:   1000,
			},
			Image{
				Alt: "whitehelmet_store_1000px.jpg",
				Url: "/img/products/whitehelmet_store_1000px.jpg",
				X:   1000,
				Y:   1000,
			},
		},
	})

	// T-Shirts
	variants = []ProductVariant{
		ProductVariant{
			SKU:   "SKULL-TSHIRT-MEN-S",
			Size:  "Men's Small",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-MEN-M",
			Size:  "Men's Medium",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-MEN-L",
			Size:  "Men's Large",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-MEN-XL",
			Size:  "Men's X-Large",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-MEN-XXL",
			Size:  "Men's XX-Large",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULL-TSHIRT-WOMEN-XS",
			Size:  "Women's X-Small",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULL-TSHIRT-WOMEN-S",
			Size:  "Women's Small",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-WOMEN-M",
			Size:  "Women's Medium",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-WOMEN-L",
			Size:  "Women's Large",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-WOMEN-XL",
			Size:  "Women's X-Large",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-WOMEN-XXL",
			Size:  "Women's XX-Large",
			Color: "Black",
			Price: 1999 * 100,
		},
	}

	for _, v := range variants {
		db.PutKey("variant", v.SKU, &v)
	}

	db.PutKey("product", "t-shirt", &Product{
		Slug:        "t-shirt",
		Title:       "SKULLY T-shirt",
		Description: "",
		Variants:    variants,
		HeaderImage: Image{
			Alt: "SKULLY T-SHIRT",
			Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/t/544011e7e4b0ea72c07a5fec/1413485036166/140919%20CoverPhoto5.jpg",
			X:   1000,
			Y:   369,
		},
		Images: []Image{
			Image{
				Alt: "skully_shirt_1000px.jpg",
				Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544f7b03e4b07cd673960362/1414494980796/skully_shirt_1000px.jpg",
				X:   1000,
				Y:   1000,
			},
		},
	})

	// Hat
	variants = []ProductVariant{
		ProductVariant{
			SKU:   "SKULLY-HAT-S",
			Size:  "S",
			Color: "Black",
			Price: 1499 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-HAT-M",
			Size:  "M",
			Color: "Black",
			Price: 1499 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-HAT-L",
			Size:  "L",
			Color: "Black",
			Price: 1499 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-HAT-L",
			Size:  "XL",
			Color: "Black",
			Price: 1499 * 100,
		},
	}

	for _, v := range variants {
		db.PutKey("variant", v.SKU, &v)
	}

	db.PutKey("product", "hat", &Product{
		Slug:        "hat",
		Title:       "SKULLY Hat",
		Description: "",
		Variants:    variants,
		HeaderImage: Image{
			Alt: "SKULLY HAT",
			Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/t/544011e7e4b0ea72c07a5fec/1413485036166/140919%20CoverPhoto5.jpg",
			X:   1000,
			Y:   369,
		},
		Images: []Image{
			Image{
				Alt: "skully_hat_1000px.jpg",
				Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544f9301e4b070a33c5fd494/1414501121892/skully_hat1000px.jpg",
				X:   1000,
				Y:   1000,
			},
		},
	})

	// Stickers
	variants = []ProductVariant{
		ProductVariant{
			SKU:   "SKULLY-STICKERS",
			Size:  "",
			Color: "",
			Price: 499 * 100,
		},
	}

	for _, v := range variants {
		db.PutKey("variant", v.SKU, &v)
	}

	db.PutKey("product", "stickers", &Product{
		Slug:        "stickers",
		Title:       "SKULLY Stickers",
		Description: "",
		Variants:    variants,
		HeaderImage: Image{
			Alt: "SKULLY STICKERS",
			Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/t/53f0cd31e4b05292018da5e2/1408290101751/motorcyclist.jpg",
			X:   1500,
			Y:   583,
		},
		Images: []Image{
			Image{
				Alt: "sticker_pack_1000px.jpg",
				Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544f9403e4b08f5872d5e730/1414501383224/sticker_pack_1000px.jpg",
				X:   1000,
				Y:   1000,
			},
		},
	})

	// Try to import existing contributors, but only if we have yet to
	count, err := db.Query("contribution").KeysOnly().Count(db.Context)
	log.Debug("Contributions persisted: %v", count)
	if count > 0 {
		return
	}
	if err != nil {
		log.Fatal("Failed to query for contributions: %v", err)
	}

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
	//	                                    Shipping Country         17
	for i := 0; true; i++ {
		// Loop until exhausted
		row, err := reader.Read()
		if err != nil {
			break
		}

		// Only save first 100 in production
		if config.Get().Development && i > 25 {
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

		// Create token
		token := new(InviteToken)
		token.Id = row[0]
		token.Email = email
		db.PutKey("invite-token", token.Id, token)

		// Save contribution
		contribution := Contribution{
			Perk:          perks[perkId],
			Status:        row[3],
			FundingDate:   row[4],
			PaymentMethod: row[5],
			Email:         email,
		}
		db.PutKey("contribution", contribution.Email, &contribution)

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
}
