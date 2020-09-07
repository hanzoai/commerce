package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/product"
	"hanzo.io/models/types/currency"
)

var _ = New("karma-trikini", func(c *gin.Context) []*product.Product {
	db := datastore.New(c)

	org := organization.New(db)
	org.Query().Filter("Name=", "karma").Get()

	nsdb := datastore.New(org.Namespaced(db.Context))

	SIZES := []size{
		size{
			id:   "XS",
			name: "XS",
		},
		size{
			id:   "S",
			name: "S",
		},
		size{
			id:   "M",
			name: "M",
		},
		size{
			id:   "L",
			name: "L",
		},
		size{
			id:   "XL",
			name: "XL",
		},
		size{
			id:   "XXL",
			name: "XXL",
		},
	}

	TOP_STYLES := []size{
		size{
			name: "Railay Bikini in trippy leopard",
			id:   "railay-top-tl",
		},
		size{
			name: "Railay Bikini in dragon blossom",
			id:   "railay-top-db",
		},
		size{
			name: "Ruby Bikini in trippy leopard",
			id:   "ruby-top-tl",
		},
		size{
			name: "Ruby Bikini in dragon blossom",
			id:   "ruby-top-db",
		},
		size{
			name: "Lafayette Bikini in trippy leopard",
			id:   "lafayette-top-tl",
		},
		size{
			name: "Lafayette Bikini in dragon blossom",
			id:   "lafayette-top-db",
		},
		size{
			name: "Bikini-n-Chill Bikini in trippy leopard",
			id:   "bikini-n-chill-top-tl",
		},
		size{
			name: "Bikini-n-Chill Bikini in dragon blossom",
			id:   "bikini-n-chill-top-db",
		},
	}

	BOTTOM_STYLES := []size{
		size{
			name: "Railay Bikini in trippy leopard",
			id:   "railay-btm-tl",
		},
		size{
			name: "Railay Bikini in dragon blossom",
			id:   "railay-btm-db",
		},
		size{
			name: "Ruby Bikini in trippy leopard",
			id:   "ruby-btm-tl",
		},
		size{
			name: "Ruby Bikini in dragon blossom",
			id:   "ruby-btm-db",
		},
		size{
			name: "Lafayette Bikini in trippy leopard",
			id:   "lafayette-btm-tl",
		},
		size{
			name: "Lafayette Bikini in dragon blossom",
			id:   "lafayette-btm-db",
		},
		size{
			name: "Bikini-n-Chill Bikini in trippy leopard",
			id:   "bikini-n-chill-btm-tl",
		},
		size{
			name: "Bikini-n-Chill Bikini in dragon blossom",
			id:   "bikini-n-chill-btm-db",
		},
	}

	MASK_STYLES := []size{
		size{
			name: "Trippy Leopard Mask",
			id:   "mask-trippy-leopard",
		},
		size{
			name: "Less Boring Summer Mask",
			id:   "mask-less-boring-summer",
		},
	}

	prods := []*product.Product{}

	for _, s1 := range TOP_STYLES {
		for _, s2 := range SIZES {
			for _, s3 := range BOTTOM_STYLES {
				for _, s4 := range SIZES {
					for _, s5 := range MASK_STYLES {
						prod := product.New(nsdb)
						prod.Slug = "trikini-" + s1.id + "-" + s2.id + "-" + s3.id + "-" + s4.id + "-" + s5.id
						prod.GetOrCreate("Slug=", prod.Slug)
						prod.Name = "Trikini " + s1.name + " " + s2.name + " Top " + s3.name + " " + s4.name + " Bottom " + s5.name + " Mask"
						prod.Description = "Guess what it’s 2020 and the only way to look cute and safe at the beach is with your bikini and mask, a.k.a. the tri-kini matching set. Choose a bikini style from the Less Boring Summer Collection and any mask. Available in our Trippy Leopard print/Dragon Blossom print. "
						prod.Currency = currency.USD
						prod.ListPrice = currency.Cents(23500)
						prod.Price = currency.Cents(23500)
						prod.Update()

						prods = append(prods, prod)
					}
				}
			}
		}
	}

	return prods
})
