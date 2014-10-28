package fixtures

import (
	"crowdstart.io/datastore"
	. "crowdstart.io/models"
)

func Install(db *datastore.Datastore) {
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
		},
		Images: []Image{
			Image{
				Alt: "blackhelmet_store.jpg",
				Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544c1bd6e4b07de01f6f22aa/1414274007569/blackhelmet_store.jpg",
			},
			Image{
				Alt: "whitehelmet_store.jpg",
				Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544c1bdde4b07de01f6f22b5/1414274015307/whitehelmet_store.jpg",
			},
		},
	})

	// T-Shirts
	variants = []ProductVariant{
		ProductVariant{
			SKU:   "SKULLY-T-SHIRT-S",
			Size:  "S",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-T-SHIRT-M",
			Size:  "M",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-T-SHIRT-L",
			Size:  "L",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-T-SHIRT-XL",
			Size:  "XL",
			Color: "Black",
			Price: 1999 * 100,
		},
		ProductVariant{
			SKU:   "SKULLY-T-SHIRT-XXL",
			Size:  "XXL",
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
		Images: []Image{
			Image{
				Alt: "skully_shirt_1000px.jpg",
				Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544f7b03e4b07cd673960362/1414494980796/skully_shirt_1000px.jpg",
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
	}

	for _, v := range variants {
		db.PutKey("variant", v.SKU, &v)
	}

	db.PutKey("product", "hat", &Product{
		Slug:        "hat",
		Title:       "SKULLY Hat",
		Description: "SKULLY Nation Hat",
		Variants:    variants,
		Images: []Image{
			Image{
				Alt: "skully_hat_1000px.jpg",
				Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544f9301e4b070a33c5fd494/1414501121892/skully_hat1000px.jpg",
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
		Images: []Image{
			Image{
				Alt: "sticker_pack_1000px.jpg",
				Url: "//static.squarespace.com/static/53dd2a15e4b06cbe07110bd5/544a257de4b015b5ef71847c/544f9403e4b08f5872d5e730/1414501383224/sticker_pack_1000px.jpg",
			},
		},
	})
}
