package fixtures

import (
	"code.google.com/p/go.crypto/bcrypt"
	"crowdstart.io/datastore"
	. "crowdstart.io/models"
)

func Install(db *datastore.Datastore) {
	pwhash, _ := bcrypt.GenerateFromPassword([]byte("password"), 12)

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
		Description: "The World's smartest helmet. Even more descriptive text.",
		Variants:    variants,
		HeaderImage: Image{
			Alt: "SKULLY AR-1",
			Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544a2657e4b0ff95316b8ea0/1414359306658/",
			X:   1500,
			Y:   844,
		},
		Images: []Image{
			Image{
				Alt: "blackhelmet_store.jpg",
				Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544c1bd6e4b07de01f6f22aa/1414274007569/blackhelmet_store.jpg",
				X:   1000,
				Y:   1000,
			},
			Image{
				Alt: "whitehelmet_store.jpg",
				Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544c1bdde4b07de01f6f22b5/1414274015307/whitehelmet_store.jpg",
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
		Description: "SKULLY Nation T-shirt",
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
		Description: "SKULLY Nation Hat",
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
		Description: "SKULLY Nation Stickers",
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
}
