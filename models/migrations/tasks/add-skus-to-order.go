package tasks

import (
	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

var AddSkusToOrder = parallel.Task("add-skus-to-order", func(db *datastore.Datastore, key datastore.Key, order models.Order) {
	for i, item := range order.Items {
		if item.SKU_ == "" {
			switch item.Slug_ {
			case "ar-1":
				order.Items[i].SKU_ = "AR-1-BLACK-M"
			case "t-shirt":
				order.Items[i].SKU_ = "SKULLY-TSHIRT-MEN-M"
			case "hat":
				order.Items[i].SKU_ = "SKULLY-HAT-M"
			case "card-winter2014promo":
				order.Items[i].SKU_ = "CARD-WINTER2014PROMO"
			case "dogtag-winter2014promo":
				order.Items[i].SKU_ = "DOGTAG-WINTER2014PROMO"
			case "stickers":
				order.Items[i].SKU_ = "SKULLY-STICKERS"
			}
			order.Unconfirmed = true
		}
	}

	if order.Unconfirmed {
		if _, err := db.Put(key, &order); err != nil {
			log.Error("%v", err)
		}
	}
})
