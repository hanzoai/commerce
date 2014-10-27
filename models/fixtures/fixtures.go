package fixtures

import (
	"crowdstart.io/datastore"
	. "crowdstart.io/models"
)

func Install(db *datastore.Datastore) {
	db.Put("product", &Product{
		Slug:  "ar-1",
		Title: "SKULLY AR-1",
		Description: "World's smartest helmet!",
	})

	variants := []ProductVariant{
		ProductVariant{
			SKU: "AR-1-BLACK-S",
			Size: "S",
			Color: "Black",
			Price: 1499*100*100,
		},
		ProductVariant{
			SKU: "AR-1-BLACK-M",
			Size: "M",
			Color: "Black",
			Price: 1499*100*100,
		},
		ProductVariant{
			SKU: "AR-1-BLACK-L",
			Size: "L",
			Color: "Black",
			Price: 1499*100*100,
		},
		ProductVariant{
			SKU: "AR-1-BLACK-XL",
			Size: "XL",
			Color: "Black",
			Price: 1499*100*100,
		},
		ProductVariant{
			SKU: "AR-1-BLACK-XXL",
			Size: "XXL",
			Color: "Black",
			Price: 1499*100*100,
		},
		ProductVariant{
			SKU: "AR-1-WHITE-S",
			Size: "S",
			Color: "White",
			Price: 1499*100*100,
		},
		ProductVariant{
			SKU: "AR-1-WHITE-M",
			Size: "M",
			Color: "White",
			Price: 1499*100*100,
		},
		ProductVariant{
			SKU: "AR-1-WHITE-L",
			Size: "L",
			Color: "White",
			Price: 1499*100*100,
		},
		ProductVariant{
			SKU: "AR-1-WHITE-XL",
			Size: "XL",
			Color: "White",
			Price: 1499*100*100,
		},
		ProductVariant{
			SKU: "AR-1-WHITE-XXL",
			Size: "XXL",
			Color: "White",
			Price: 1499*100*100,
		},
	}

	for _, v := range variants {
		db.Put("variant", &v)
	}
}
