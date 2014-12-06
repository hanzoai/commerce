package fixtures

import (
	"appengine"
	"appengine/delay"
	"encoding/csv"
	"os"
	"strings"

	"code.google.com/p/go.crypto/bcrypt"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/util/log"

	. "crowdstart.io/models"
)

var All = delay.Func("install-all-fixtures", func(c appengine.Context) {
	log.Debug("Loading fixtures...")
	db := datastore.New(c)

	pwhash, _ := bcrypt.GenerateFromPassword([]byte("Victory1!"), 12)

	// Default User (SKULLY)
	db.PutKey("user", "dev@hanzo.ai", &User{
		Id:           "dev@hanzo.ai",
		FirstName:    "Mitchell",
		LastName:     "Weller",
		Email:        "dev@hanzo.ai",
		Phone:        "(123) 456-7890",
		OrdersIds:    []string{},
		PasswordHash: pwhash,
	})

	// Default Campaign (SKULLY)
	db.PutKey("campaign", "dev@hanzo.ai", &Campaign{
		Id:    "dev@hanzo.ai",
		Title: "SKULLY AR-1",
	})

	// AR-1
	variants := []ProductVariant{
		ProductVariant{
			SKU:   "AR-1-BLACK-S",
			Size:  "S",
			Color: "Matte Black",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-BLACK-M",
			Size:  "M",
			Color: "Matte Black",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-BLACK-L",
			Size:  "L",
			Color: "Matte Black",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-BLACK-XL",
			Size:  "XL",
			Color: "Matte Black",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-BLACK-XXL",
			Size:  "XXL",
			Color: "Matte Black",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-WHITE-S",
			Size:  "S",
			Color: "Gloss White",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-WHITE-M",
			Size:  "M",
			Color: "Gloss White",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-WHITE-L",
			Size:  "L",
			Color: "Gloss White",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-WHITE-XL",
			Size:  "XL",
			Color: "Gloss White",
			Price: 1499 * 100 * 100,
		},
		ProductVariant{
			SKU:   "AR-1-WHITE-XXL",
			Size:  "XXL",
			Color: "Gloss White",
			Price: 1499 * 100 * 100,
		},
	}

	for _, v := range variants {
		db.PutKey("variant", v.SKU, &v)
	}

	db.PutKey("product", "ar-1", &Product{
		Slug:     "ar-1",
		Title:    "SKULLY AR-1",
		Headline: "The World's smartest helmet.",
		Excerpt:  "The World's smartest motorcycle helmet.",
		Description: `The world’s smartest motorcycle helmet. SKULLY AR-1 is a light, high-quality,
					  and full-faced motorcycle helmet equipped with a wide-angle rearview camera and
					  transparent heads up display (HUD). With its live rearview feed and ability to
					  provide telemetry and rider data such as speed, GPS directions, fuel*, and
					  more, the SKULLY AR-1 not only eliminates blind spots, but allows the rider to
					  focus on what matters most: the road ahead. SKULLY AR-1: Ride safer, look
					  badass.`,
		Variants: variants,
		Disabled: true,
		HeaderImage: Image{
			Alt: "SKULLY AR-1",
			Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544a2657e4b0ff95316b8ea0/1414359306658/",
			X:   1500,
			Y:   844,
		},
		Images: []Image{
			Image{
				Alt: "blackhelmet_store_1000px.jpg",
				Url: config.UrlFor("/img/products/blackhelmet_store_1000px.jpg"),
				X:   1000,
				Y:   1000,
			},
			Image{
				Alt: "whitehelmet_store_1000px.jpg",
				Url: config.UrlFor("/img/products/whitehelmet_store_1000px.jpg"),
				X:   1000,
				Y:   1000,
			},
		},
	})

	// Cards
	variants = []ProductVariant{
		ProductVariant{
			SKU:   "CARD-WINTER2014PROMO",
			Price: 0,
		},
	}

	for _, v := range variants {
		db.PutKey("variant", v.SKU, &v)
	}

	db.PutKey("product", "card-winter2014promo", &Product{
		Slug:     "card-winter2014promo",
		Title:    "SKULLY X-mas Card",
		Variants: variants,
		Disabled: true,
		Images: []Image{ // replace with real one, zach
			Image{
				Alt: "whitehelmet_store_1000px.jpg",
				Url: config.UrlFor("/img/products/whitehelmet_store_1000px.jpg"),
				X:   1000,
				Y:   1000,
			},
		},
	})

	// Dogtags
	variants = []ProductVariant{
		ProductVariant{
			SKU:   "DOGTAG-WINTER2014PROMO",
			Price: 0,
		},
	}

	for _, v := range variants {
		db.PutKey("variant", v.SKU, &v)
	}

	db.PutKey("product", "dogtag-winter2014promo", &Product{
		Slug:     "dogtag-winter2014promo",
		Title:    "SKULLY X-mas Dogtag",
		Variants: variants,
		Disabled: true,
		Images: []Image{ // replace with real one, zach
			Image{
				Alt: "whitehelmet_store_1000px.jpg",
				Url: config.UrlFor("/img/products/whitehelmet_store_1000px.jpg"),
				X:   1000,
				Y:   1000,
			},
		},
	})

	// T-Shirts
	variants = []ProductVariant{
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-MEN-XS",
			Style: "Men's T-Shirt",
			Size:  "XS",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-MEN-S",
			Style: "Men's T-Shirt",
			Size:  "S",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-MEN-M",
			Style: "Men's T-Shirt",
			Size:  "M",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-MEN-L",
			Style: "Men's T-Shirt",
			Size:  "L",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-MEN-XL",
			Style: "Men's T-Shirt",
			Size:  "XL",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-MEN-XXL",
			Style: "Men's T-Shirt",
			Size:  "XXL",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-MEN-XXXL",
			Style: "Men's T-Shirt",
			Size:  "XXXL",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-WOMEN-XS",
			Style: "Women's T-Shirt",
			Size:  "XS",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-WOMEN-S",
			Style: "Women's T-Shirt",
			Size:  "S",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-WOMEN-M",
			Style: "Women's T-Shirt",
			Size:  "M",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-WOMEN-L",
			Style: "Women's T-Shirt",
			Size:  "L",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-TSHIRT-WOMEN-XL",
			Style: "Women's T-Shirt",
			Size:  "XL",
			Color: "Black",
			Price: 1999 * 100,
		},
	}

	for _, v := range variants {
		db.PutKey("variant", v.SKU, &v)
	}

	db.PutKey("product", "t-shirt", &Product{
		Slug:    "t-shirt",
		Title:   "SKULLY T-shirt",
		Excerpt: "Rock your SKULLY Nation pride with our official Team SKULLY t-shirt.",
		Description: `Rock your SKULLY Nation pride with our official Team SKULLY t-shirt, the
					  perfect way to rep your status as a rebel innovator and modern badass. Our
					  premium quality tee comes in all sizes, with fits for both the dapper male and
					  stylish female rider.`,
		Variants: variants,
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
			SKU:   "SKULLY-HAT-XL",
			Size:  "XL",
			Color: "Black",
			Price: 1499 * 100,
		},
	}

	for _, v := range variants {
		db.PutKey("variant", v.SKU, &v)
	}

	db.PutKey("product", "hat", &Product{
		Slug:    "hat",
		Title:   "SKULLY Hat",
		Excerpt: "Look like a badass in our official SKULLY embroidered 6-panel flexible fitted cap.",
		Description: `Look like a badass in our official SKULLY embroidered 6-panel flexible fitted
					  cap, a hat designed with your comfort in mind. The pro-stitching on the crown
					  will see your SKULLY hat through rain or shine, while its 6 embroidered eyelets
					  will keep your dome well-ventilated and cool.`,
		Variants: variants,
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
		Slug:    "stickers",
		Title:   "SKULLY Stickers",
		Excerpt: "No laptop or motorcycle is complete without a premium vinyl SKULLY sticker stretched across it.",
		Description: `No laptop, motorcycle, or forehead is complete without a premium vinyl SKULLY
					 sticker stretched across it (okay, maybe not your forehead). Slap these babies
					 on your helmets, tablets, even your desk, and enjoy the flood of comments that
					 will quickly follow when your fellow rebel innovators recognize you as part of
					 the SKULLY Nation elite.`,
		Variants: variants,
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

	// Product Listings

	db.PutKey("productlisting", "ar-1-winter2014promo", &ProductListing{
		Slug:     "ar-1-winter2014promo",
		Title:    "SKULLY AR-1",
		Headline: "The World's smartest helmet.",
		Excerpt:  "The World's smartest motorcycle helmet.",
		Description: `The world’s smartest motorcycle helmet. SKULLY AR-1 is a light, high-quality,
					  and full-faced motorcycle helmet equipped with a wide-angle rearview camera and
					  transparent heads up display (HUD). With its live rearview feed and ability to
					  provide telemetry and rider data such as speed, GPS directions, fuel*, and
					  more, the SKULLY AR-1 not only eliminates blind spots, but allows the rider to
					  focus on what matters most: the road ahead. SKULLY AR-1: Ride safer, look
					  badass.

					  *Pre-Order during the holiday season for a FREE LIMITED EDITION SKULLY AR-1 dogtag & XMAS Card`,
		CheckOutInstructions: "*FREE dogtag & X-mas card added at checkout",
		Images: []Image{
			Image{
				Alt: "blackhelmet_store_1000px.jpg",
				Url: config.UrlFor("/img/products/blackhelmet_store_1000px.jpg"),
				X:   1000,
				Y:   1000,
			},
			Image{
				Alt: "whitehelmet_store_1000px.jpg",
				Url: config.UrlFor("/img/products/whitehelmet_store_1000px.jpg"),
				X:   1000,
				Y:   1000,
			},
		},
		ProductConfigs: []ProductConfig{
			ProductConfig{
				Product:  "card-winter2014promo",
				Quantity: 1,
			},
			ProductConfig{
				Product:  "dogtag-winter2014promo",
				Quantity: 1,
			},
		},
	})

	// Users

	if count, _ := db.Query("user").Count(c); count > 1 {
		log.Debug("Contributor fixtures already loaded, skipping.")
		return
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
		// Only save first 25 in development
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

		// Create token
		token := new(InviteToken)
		token.Id = tokenId
		token.Email = email
		db.PutKey("invite-token", tokenId, token)

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

		// No longer updating user information in production, as it would clobber any customized information.
		if config.IsProduction {
			return
		} else {
			db.PutKey("user", user.Email, user)
		}

		log.Debug("User %#v", user)
		log.Debug("InviteToken: %#v", token)
	}
})
