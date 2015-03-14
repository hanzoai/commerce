package models

type Perk struct {
	Id                string
	Description       string
	EstimatedDelivery string
	GearQuantity      int
	HelmetQuantity    int
	Price             string
	Title             string
}

var Perks = map[string]Perk{
	"2210257": Perk{
		Id:                "2210257",
		Title:             "Speed Demon SKULLY AR-1",
		Description:       "One of the first 25 riders to get a SKULLY AR-1 Motorcycle helmet at an unbeatable price. Free shipping to the United States. Sizes S-XXL in Matte Black or Gloss White.",
		Price:             "$1,299 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    1,
		GearQuantity:      1,
	},

	"2238897": Perk{
		Id:                "2238897",
		Title:             "Speed Demon SKULLY AR-1",
		Description:       "One of the first 25 riders to get a SKULLY AR-1 Motorcycle helmet at an unbeatable price. Free shipping to the United States. Sizes S-XXL in Matte Black or Gloss White.",
		Price:             "$1,299 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    1,
		GearQuantity:      1,
	},

	"2123732": Perk{
		Id:                "2123732",
		Title:             "SKULLY AR-1",
		Description:       "Get 1 SKULLY AR-1 Motorcycle helmet at a limited introductory launch price. Free shipping to the United States. Sizes S-XXL in Matte Black or Gloss White.",
		Price:             "$1,399 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    1,
		GearQuantity:      1,
	},

	"2291929": Perk{
		Id:                "2291929",
		Title:             "SKULLY NATION GEAR",
		Description:       "Join Team SKULLY with a package of official gear: limited edition hat, shirt, and decals. Free shipping to the United States. Shirt sizes S-XXL (available in men’s and women’s cut), Flexfit hat sizes S/M, L/XL. *Free w/ AR-1 preorder.",
		Price:             "$49 USD",
		EstimatedDelivery: "December 2014",
		HelmetQuantity:    0,
		GearQuantity:      1,
	},

	"2267279": Perk{
		Id:                "2267279",
		Title:             "$499 Now and $949 Due at Ship",
		Description:       "Reserve 1 SKULLY AR-1 Motorcycle helmet with $499 deposit, and $949* due when the AR-1 ships. Sizes S-XXL in Matte Black or Gloss White. Reserved helmets will ship after all regular pre-orders are shipped. Free shipping to the United States. *FOR INTERNATIONAL: additional shipping fee of $99.99 USD will apply to the second payment. We will email you closer to your ship date to collect the second payment.",
		Price:             "$499 USD",
		EstimatedDelivery: "July 2015",
		HelmetQuantity:    1,
		GearQuantity:      1,
	},

	"2210221": Perk{
		Id:                "2210221",
		Title:             "International SKULLY AR-1",
		Description:       "Includes international shipping. Get 1 SKULLY AR-1 Motorcycle helmet at our international launch price. Sizes S-XXL in Matte Black or Gloss White.",
		Price:             "$1,599 USD ($100 refund at shipment)",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    1,
		GearQuantity:      1,
	},

	"2244337": Perk{
		Id:                "2244337",
		Title:             "International SKULLY AR-1",
		Description:       "Includes international shipping. Get 1 SKULLY AR-1 Motorcycle helmet at our international launch price. Sizes S-XXL in Matte Black or Gloss White.",
		Price:             "$1,499 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    1,
		GearQuantity:      1,
	},

	"2123523": Perk{
		Id:                "2123523",
		Title:             "Signature Edition SKULLY AR-1",
		Description:       "Get 1 Signature Edition SKULLY AR-1 Motorcycle helmet, hand-numbered and signed by CEO Marcus Weller. Sizes S-XXL in Matte Black or Gloss White.",
		Price:             "$1,999 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    1,
		GearQuantity:      1,
	},

	"2210249": Perk{
		Id:                "2210249",
		Title:             "Passenger 2-Pack Deal",
		Description:       "Get 2 SKULLY AR-1 Motorcycle helmets and save! Sizes S-XXL in Matte Black or Gloss White.",
		Price:             "$2,649 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    2,
		GearQuantity:      2,
	},

	"2291934": Perk{
		Id:                "2291934",
		Title:             "CLUB & DISTRIBUTOR 5 PACK",
		Description:       "Outfit your crew with 5 SKULLY AR-1s and save even more! Free shipping to the United States. Sizes S-XXL in Matte Black or Gloss White. *FOR INTERNATIONAL, add $375 USD for shipping.",
		Price:             "$6,495 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    5,
		GearQuantity:      5,
	},

	"2353031": Perk{
		Id:                "2353031",
		Title:             "20 AR-1 DISTRIBUTOR PACK",
		Description:       "Line your store shelves with 20 SKULLY AR-1s for massive savings! Free shipping to the United States. Sizes S-XXL in Matte Black or Gloss White. *FOR INTERNATIONAL, add $1499 USD for shipping.",
		Price:             "$24,979 USD",
		EstimatedDelivery: "May 2015",
		HelmetQuantity:    20,
		GearQuantity:      20,
	},

	"WINTER2014PROMO": Perk{
		Id:                "WINTER2014PROMO",
		Title:             "AR-1 HOLIDAY PREORDER",
		Description:       "Get 1 SKULLY AR-1 Motorcycle helmet, a LIMITED EDITION Holiday dogtag, and downloadable X-mas card. Free shipping to the United States. Sizes S-XXL in Matte Black or Gloss White.",
		Price:             "$1,499 USD",
		EstimatedDelivery: "HELMET: May 2015,DOGTAG: December 2014,XMAS CARD: Downloadable",
		HelmetQuantity:    1,
		GearQuantity:      0,
	},
}
