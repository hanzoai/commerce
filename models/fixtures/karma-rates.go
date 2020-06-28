package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/shippingrates"
	"hanzo.io/models/store"
	"hanzo.io/models/taxrates"
	"hanzo.io/models/types/georate"
)

var _ = New("karma", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "karma"
	org.GetOrCreate("Name=", org.Name)

	nsdb := datastore.New(org.Namespaced(db.Context))

	// Create default store
	stor := store.New(nsdb)
	stor.Name = "Default"
	stor.GetOrCreate("Name=", stor.Name)

	trs, _ := stor.GetTaxRates()
	trs.GeoRates = []taxrates.GeoRate{
		taxrates.GeoRate{
			GeoRate: georate.New(
				"US",
				"CA",
				"",
				"64108",
				0,
				0,
				0,
				0,
			),
		},
	}

	trs.MustUpdate()

	srs, _ := stor.GetShippingRates()
	srs.GeoRates = []shippingrates.GeoRate{
		shippingrates.GeoRate{
			GeoRate: georate.New(
				"",
				"",
				"",
				"",
				15000,
				0,
				0,
				0,
			),
		},
		shippingrates.GeoRate{
			GeoRate: georate.New(
				"",
				"",
				"",
				"",
				0,
				15000,
				0,
				4500,
			),
		},
		shippingrates.GeoRate{
			GeoRate: georate.New(
				"US",
				"",
				"",
				"",
				15000,
				0,
				0,
				0,
			),
		},
		shippingrates.GeoRate{
			GeoRate: georate.New(
				"US",
				"",
				"",
				"",
				0,
				15000,
				0,
				400,
			),
		},
	}

	return org
})
