package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	. "crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/task"
)

var listings = task.Func("fixtures-listings", func(c *gin.Context) {
	log.Debug("Loading fixtures...")
	db := datastore.New(c)

	// Product Listings
	db.PutKind("listing", "ar-1-winter2014promo", &Listing{
		SKU:   "ar-1-winter2014promo",
		Title: "SKULLY AR-1",
		Description: `The world’s smartest motorcycle helmet. SKULLY AR-1 is a light, high-quality,
					  and full-faced motorcycle helmet equipped with a wide-angle rearview camera and
					  transparent heads up display (HUD). With its live rearview feed and ability to
					  provide telemetry and rider data such as speed, GPS directions, fuel, and
					  more, the SKULLY AR-1 not only eliminates blind spots, but allows the rider to
					  focus on what matters most: the road ahead. SKULLY AR-1: Ride safer, look
					  badass.

					  Estimated Delivery: JULY 2015

					  *Pre-Order during the holiday season for a FREE LIMITED EDITION SKULLY AR-1 dog tag & XMAS Card`,
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
		EstimatedDelivery: "July 2015",
		Disabled:          true,
		SoldOut:           true,
		Configs: []Config{
			Config{
				Product:  "ar-1",
				Quantity: 1,
			},
			Config{
				Product:  "card-winter2014promo",
				Variant:  "CARD-WINTER2014PROMO",
				Quantity: 1,
			},
			Config{
				Product:  "dogtag-winter2014promo",
				Variant:  "DOGTAG-WINTER2014PROMO",
				Quantity: 1,
			},
		},
	})

	db.PutKind("listing", "ar-1", &Listing{
		SKU:   "ar-1",
		Title: "SKULLY AR-1",
		Description: `The world’s smartest motorcycle helmet. SKULLY AR-1 is a light, high-quality,
					  and full-faced motorcycle helmet equipped with a wide-angle rearview camera and
					  transparent heads up display (HUD). With its live rearview feed and ability to
					  provide telemetry and rider data such as speed, GPS directions, fuel, and
					  more, the SKULLY AR-1 not only eliminates blind spots, but allows the rider to
					  focus on what matters most: the road ahead. SKULLY AR-1: Ride safer, look
					  badass.`,
		Images: []Image{
			Image{
				Alt: "blackhelmet_store.png",
				Url: config.UrlFor("/img/products/blackhelmet_store.png"),
				X:   1000,
				Y:   1000,
			},
			Image{
				Alt: "whitehelmet_store.png",
				Url: config.UrlFor("/img/products/whitehelmet_store.png"),
				X:   1000,
				Y:   1000,
			},
		},
		EstimatedDelivery: "Late 2015",
		Configs: []Config{
			Config{
				Product:  "ar-1",
				Quantity: 1,
			},
		},
	})
})
