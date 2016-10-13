package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/product"
	"crowdstart.com/models/types/currency"
)

var LudelaProd = New("ludela", func(c *gin.Context) []*product.Product {
	db := datastore.New(c)

	org := organization.New(db)
	org.Query().Filter("Name=", "ludela").Get()

	nsdb := datastore.New(org.Namespaced(db.Context))

	// Create smart candle
	prod1 := product.New(nsdb)
	prod1.Slug = "Solo-W-I"
	prod1.GetOrCreate("Slug=", prod1.Slug)
	prod1.SetKey("Knc9wlZJUOOG")
	prod1.Name = "Solo Starter Kit, Ivory Wax Shell"
	prod1.Description = "Includes: One (1) LuDela Smart Candle, Ivory Color, One (1) 100% Natural Soy-Beeswax Refill (30-hour burn time). Special Offer: One-month trial subscription to LuDela’s EssentialRefill Program of two (2) 30-hour refills per month, per LuDela ordered. $9.99 per month per LuDela thereafter. Modify or cancel anytime."
	prod1.Currency = currency.USD
	prod1.ListPrice = currency.Cents(19900)
	prod1.Price = currency.Cents(9900)
	prod1.Preorder = true
	prod1.Hidden = false
	prod1.EstimatedDelivery = "Early 2017"
	prod1.Update()

	prod2 := product.New(nsdb)
	prod2.Slug = "Solo-W-O"
	prod2.GetOrCreate("Slug=", prod2.Slug)
	prod2.SetKey("Knc9wlZJUFJE")
	prod2.Name = "Solo Starter Kit, Orange Wax Shell Upgrade"
	prod2.Description = "Includes: One (1) LuDela Smart Candle, Orange Color, One (1) 100% Natural Soy-Beeswax Refill (30-hour burn time). Special Offer: One-month trial subscription to LuDela’s EssentialRefill Program of two (2) 30-hour refills per month, per LuDela ordered. $9.99 per month per LuDela thereafter. Modify or cancel anytime."
	prod2.Currency = currency.USD
	prod2.ListPrice = currency.Cents(22900)
	prod2.Price = currency.Cents(11900)
	prod2.Preorder = true
	prod2.Hidden = false
	prod2.EstimatedDelivery = "Early 2017"
	prod2.Update()

	prod3 := product.New(nsdb)
	prod3.Slug = "Solo-W-R"
	prod3.GetOrCreate("Slug=", prod3.Slug)
	prod3.SetKey("Knc9wlZFJEJE")
	prod3.Name = "Solo Starter Kit, Red Wax Shell Upgrade"
	prod3.Description = "Includes: One (1) LuDela Smart Candle, Red Color, One (1) 100% Natural Soy-Beeswax Refill (30-hour burn time). Special Offer: One-month trial subscription to LuDela’s EssentialRefill Program of two (2) 30-hour refills per month, per LuDela ordered. $9.99 per month per LuDela thereafter. Modify or cancel anytime."
	prod3.Currency = currency.USD
	prod3.ListPrice = currency.Cents(22900)
	prod3.Price = currency.Cents(11900)
	prod3.Preorder = true
	prod3.Hidden = false
	prod3.EstimatedDelivery = "Early 2017"
	prod3.Update()

	prod4 := product.New(nsdb)
	prod4.Slug = "Solo-W-B"
	prod4.GetOrCreate("Slug=", prod4.Slug)
	prod4.SetKey("Knc9wlZFJEFE")
	prod4.Name = "Solo Starter Kit, Blue Wax Shell Upgrade"
	prod4.Description = "Includes: One (1) LuDela Smart Candle, Blue Color, One (1) 100% Natural Soy-Beeswax Refill (30-hour burn time). Special Offer: One-month trial subscription to LuDela’s EssentialRefill Program of two (2) 30-hour refills per month, per LuDela ordered. $9.99 per month per LuDela thereafter. Modify or cancel anytime."
	prod4.Currency = currency.USD
	prod4.ListPrice = currency.Cents(22900)
	prod4.Price = currency.Cents(11900)
	prod4.Preorder = true
	prod4.Hidden = false
	prod4.EstimatedDelivery = "Early 2017"
	prod4.Update()

	prod5 := product.New(nsdb)
	prod5.Slug = "Solo-W-G"
	prod5.GetOrCreate("Slug=", prod5.Slug)
	prod5.SetKey("Knc9wlZFJIII")
	prod5.Name = "Solo Starter Kit, Green Wax Shell Upgrade"
	prod5.Description = "Includes: One (1) LuDela Smart Candle, Green Color, One (1) 100% Natural Soy-Beeswax Refill (30-hour burn time). Special Offer: One-month trial subscription to LuDela’s EssentialRefill Program of two (2) 30-hour refills per month, per LuDela ordered. $9.99 per month per LuDela thereafter. Modify or cancel anytime."
	prod5.Currency = currency.USD
	prod5.ListPrice = currency.Cents(22900)
	prod5.Price = currency.Cents(11900)
	prod5.Preorder = true
	prod5.Hidden = false
	prod5.EstimatedDelivery = "Early 2017"
	prod5.Update()

	prod6 := product.New(nsdb)
	prod6.Slug = "Solo-W-P"
	prod6.GetOrCreate("Slug=", prod6.Slug)
	prod6.SetKey("Knc9wlZFJFFI")
	prod6.Name = "Solo Starter Kit, Purple Wax Shell Upgrade"
	prod6.Description = "Includes: One (1) LuDela Smart Candle, Purple Color, One (1) 100% Natural Soy-Beeswax Refill (30-hour burn time). Special Offer: One-month trial subscription to LuDela’s EssentialRefill Program of two (2) 30-hour refills per month, per LuDela ordered. $9.99 per month per LuDela thereafter. Modify or cancel anytime."
	prod6.Currency = currency.USD
	prod6.ListPrice = currency.Cents(22900)
	prod6.Price = currency.Cents(11900)
	prod6.Preorder = true
	prod6.Hidden = false
	prod6.EstimatedDelivery = "Early 2017"
	prod6.Update()

	prod7 := product.New(nsdb)
	prod7.Slug = "Solo-G-B"
	prod7.GetOrCreate("Slug=", prod7.Slug)
	prod7.SetKey("Knc9wlZFJFFI")
	prod7.Name = "Solo Starter Kit, Black Glass Shell Upgrade"
	prod7.Description = "Includes: One (1) LuDela Smart Candle, Black Color, Glass Shell, One (1) 100% Natural Soy-Beeswax Refill (30-hour burn time). Special Offer: One-month trial subscription to LuDela’s EssentialRefill Program of two (2) 30-hour refills per month, per LuDela ordered. $9.99 per month per LuDela thereafter. Modify or cancel anytime."
	prod7.Currency = currency.USD
	prod7.ListPrice = currency.Cents(24900)
	prod7.Price = currency.Cents(12900)
	prod7.Preorder = true
	prod7.Hidden = false
	prod7.EstimatedDelivery = "Early 2017"
	prod7.Update()

	prod8 := product.New(nsdb)
	prod8.Slug = "Solo-G-W"
	prod8.GetOrCreate("Slug=", prod8.Slug)
	prod8.SetKey("Knc9wlZFJFFI")
	prod8.Name = "Solo Starter Kit, White Glass Shell Upgrade"
	prod8.Description = "Includes: One (1) LuDela Smart Candle, White Color, Glass Shell, One (1) 100% Natural Soy-Beeswax Refill (30-hour burn time). Special Offer: One-month trial subscription to LuDela’s EssentialRefill Program of two (2) 30-hour refills per month, per LuDela ordered. $9.99 per month per LuDela thereafter. Modify or cancel anytime."
	prod8.Currency = currency.USD
	prod8.ListPrice = currency.Cents(24900)
	prod8.Price = currency.Cents(12900)
	prod8.Preorder = true
	prod8.Hidden = false
	prod8.EstimatedDelivery = "Early 2017"
	prod8.Update()

	prod9 := product.New(nsdb)
	prod9.Slug = "Solo-G-R"
	prod9.GetOrCreate("Slug=", prod9.Slug)
	prod9.SetKey("Knc9wlZFJFFI")
	prod9.Name = "Solo Starter Kit, Red Glass Shell Upgrade"
	prod9.Description = "Includes: One (1) LuDela Smart Candle, Red Color, Glass Shell, One (1) 100% Natural Soy-Beeswax Refill (30-hour burn time). Special Offer: One-month trial subscription to LuDela’s EssentialRefill Program of two (2) 30-hour refills per month, per LuDela ordered. $9.99 per month per LuDela thereafter. Modify or cancel anytime."
	prod9.Currency = currency.USD
	prod9.ListPrice = currency.Cents(24900)
	prod9.Price = currency.Cents(12900)
	prod9.Preorder = true
	prod9.Hidden = false
	prod9.EstimatedDelivery = "Early 2017"
	prod9.Update()

	prod10 := product.New(nsdb)
	prod10.Slug = "Solo-G-SM"
	prod10.GetOrCreate("Slug=", prod10.Slug)
	prod10.SetKey("Knc9wlZFJFOI")
	prod10.Name = "Solo Starter Kit, Silver Mercury Shell Upgrade"
	prod10.Description = "Includes: One (1) LuDela Smart Candle, Silver Mercury Shell, One (1) 100% Natural Soy-Beeswax Refill (30-hour burn time). Special Offer: One-month trial subscription to LuDela’s EssentialRefill Program of two (2) 30-hour refills per month, per LuDela ordered. $9.99 per month per LuDela thereafter. Modify or cancel anytime."
	prod10.Currency = currency.USD
	prod10.ListPrice = currency.Cents(24900)
	prod10.Price = currency.Cents(12900)
	prod10.Preorder = true
	prod10.Hidden = false
	prod10.EstimatedDelivery = "Early 2017"
	prod10.Update()

	prod11 := product.New(nsdb)
	prod11.Slug = "Solo-G-BM"
	prod11.GetOrCreate("Slug=", prod11.Slug)
	prod11.SetKey("Knc9wlZFkKOI")
	prod11.Name = "Solo Starter Kit, Bronze Mercury Shell Upgrade"
	prod11.Description = "Includes: One (1) LuDela Smart Candle, Bronze Mercury Shell, One (1) 100% Natural Soy-Beeswax Refill (30-hour burn time). Special Offer: One-month trial subscription to LuDela’s EssentialRefill Program of two (2) 30-hour refills per month, per LuDela ordered. $9.99 per month per LuDela thereafter. Modify or cancel anytime."
	prod11.Currency = currency.USD
	prod11.ListPrice = currency.Cents(24900)
	prod11.Price = currency.Cents(12900)
	prod11.Preorder = true
	prod11.Hidden = false
	prod11.EstimatedDelivery = "Early 2017"
	prod11.Update()

	prod12 := product.New(nsdb)
	prod12.Slug = "SC-1"
	prod12.GetOrCreate("Slug=", prod12.Slug)
	prod12.SetKey("Knc9wlKKkKOI")
	prod12.Name = "Scent: Vanilla Bliss"
	prod12.Description = "A relaxing blend of sweet, buttery vanilla with hints of coconut and tonka bean"
	prod12.Currency = currency.USD
	prod12.ListPrice = currency.Cents(700)
	prod12.Price = currency.Cents(800)
	prod12.Preorder = true
	prod12.Hidden = false
	prod12.EstimatedDelivery = "Early 2017"
	prod12.Update()

	prod13 := product.New(nsdb)
	prod13.Slug = "SC-1"
	prod13.GetOrCreate("Slug=", prod13.Slug)
	prod13.SetKey("Knc9wlKKkKOI")
	prod13.Name = "Scent: Vanilla Bliss"
	prod13.Description = "A relaxing blend of sweet, buttery vanilla with hints of coconut and tonka bean"
	prod13.Currency = currency.USD
	prod13.ListPrice = currency.Cents(700)
	prod13.Price = currency.Cents(800)
	prod13.Preorder = true
	prod13.Hidden = false
	prod13.EstimatedDelivery = "Early 2017"
	prod13.Update()

	prod14 := product.New(nsdb)
	prod14.Slug = "SC-1-r"
	prod14.GetOrCreate("Slug=", prod14.Slug)
	prod14.SetKey("Knc9wlKKkKOI")
	prod14.Name = "Scent: Vanilla Bliss (refill)"
	prod14.Description = "A relaxing blend of sweet, buttery vanilla with hints of coconut and tonka bean"
	prod14.Currency = currency.USD
	prod14.ListPrice = currency.Cents(700)
	prod14.Price = currency.Cents(800)
	prod14.Preorder = true
	prod14.Hidden = false
	prod14.EstimatedDelivery = "Early 2017"
	prod14.Update()

	prod15 := product.New(nsdb)
	prod15.Slug = "SC-2"
	prod15.GetOrCreate("Slug=", prod15.Slug)
	prod15.SetKey("Knc9wlfjwKOI")
	prod15.Name = "Scent: Dew Kissed Petal"
	prod15.Description = "A delightful blend of fruity pears and peaches with floral tons of jasmine and waterlily"
	prod15.Currency = currency.USD
	prod15.ListPrice = currency.Cents(700)
	prod15.Price = currency.Cents(800)
	prod15.Preorder = true
	prod15.Hidden = false
	prod15.EstimatedDelivery = "Early 2017"
	prod15.Update()

	prod16 := product.New(nsdb)
	prod16.Slug = "SC-2-r"
	prod16.GetOrCreate("Slug=", prod16.Slug)
	prod16.SetKey("Knc9wlFioKOI")
	prod16.Name = "Scent: Dew Kissed Petal (refill)"
	prod16.Description = "A delightful blend of fruity pears and peaches with floral tons of jasmine and waterlily"
	prod16.Currency = currency.USD
	prod16.ListPrice = currency.Cents(700)
	prod16.Price = currency.Cents(800)
	prod16.Preorder = true
	prod16.Hidden = false
	prod16.EstimatedDelivery = "Early 2017"
	prod16.Update()

	prod17 := product.New(nsdb)
	prod17.Slug = "SC-3"
	prod17.GetOrCreate("Slug=", prod17.Slug)
	prod17.SetKey("Knc99e2jwKOI")
	prod17.Name = "Scent: Lavender Escape"
	prod17.Description = "A calming blend of lavender with vanilla makes this an excellent choice to reduce stress and help you sleep better"
	prod17.Currency = currency.USD
	prod17.ListPrice = currency.Cents(700)
	prod17.Price = currency.Cents(800)
	prod17.Preorder = true
	prod17.Hidden = false
	prod17.EstimatedDelivery = "Early 2017"
	prod17.Update()

	prod18 := product.New(nsdb)
	prod18.Slug = "SC-3-r"
	prod18.GetOrCreate("Slug=", prod18.Slug)
	prod18.SetKey("Knc99e2j932I")
	prod18.Name = "Scent: Lavender Escape (refill)"
	prod18.Description = "A calming blend of lavender with vanilla makes this an excellent choice to reduce stress and help you sleep better"
	prod18.Currency = currency.USD
	prod18.ListPrice = currency.Cents(700)
	prod18.Price = currency.Cents(800)
	prod18.Preorder = true
	prod18.Hidden = false
	prod18.EstimatedDelivery = "Early 2017"
	prod18.Update()

	prod19 := product.New(nsdb)
	prod19.Slug = "SC-4"
	prod19.GetOrCreate("Slug=", prod19.Slug)
	prod19.SetKey("Knc99e2jwKOI")
	prod19.Name = "Scent: Pomegranate Delight"
	prod19.Description = "Fresh red currants and pomegranate touched with a splash of orange and finished with a twist of lemon."
	prod19.Currency = currency.USD
	prod19.ListPrice = currency.Cents(700)
	prod19.Price = currency.Cents(800)
	prod19.Preorder = true
	prod19.Hidden = false
	prod19.EstimatedDelivery = "Early 2017"
	prod19.Update()

	prod20 := product.New(nsdb)
	prod20.Slug = "SC-4-r"
	prod20.GetOrCreate("Slug=", prod20.Slug)
	prod20.SetKey("Knc9933jwKOI")
	prod20.Name = "Scent: Pomegranate Delight (refill)"
	prod20.Description = "Fresh red currants and pomegranate touched with a splash of orange and finished with a twist of lemon."
	prod20.Currency = currency.USD
	prod20.ListPrice = currency.Cents(700)
	prod20.Price = currency.Cents(800)
	prod20.Preorder = true
	prod20.Hidden = false
	prod20.EstimatedDelivery = "Early 2017"
	prod20.Update()

	prod21 := product.New(nsdb)
	prod21.Slug = "SC-5"
	prod21.GetOrCreate("Slug=", prod21.Slug)
	prod21.SetKey("Knc939sssKOI")
	prod21.Name = "Scent: Mango Driftwood"
	prod21.Description = "A perfect blend of freshly-sliced mango and oranges combined with woody basenotes of cedarwood and amber"
	prod21.Currency = currency.USD
	prod21.ListPrice = currency.Cents(700)
	prod21.Price = currency.Cents(800)
	prod21.Preorder = true
	prod21.Hidden = false
	prod21.EstimatedDelivery = "Early 2017"
	prod21.Update()

	prod22 := product.New(nsdb)
	prod22.Slug = "SC-5-r"
	prod22.GetOrCreate("Slug=", prod22.Slug)
	prod22.SetKey("Knc939sssKOI")
	prod22.Name = "Scent: Mango Driftwood (refill)"
	prod22.Description = "A perfect blend of freshly-sliced mango and oranges combined with woody basenotes of cedarwood and amber"
	prod22.Currency = currency.USD
	prod22.ListPrice = currency.Cents(700)
	prod22.Price = currency.Cents(800)
	prod22.Preorder = true
	prod22.Hidden = false
	prod22.EstimatedDelivery = "Early 2017"
	prod22.Update()

	prod23 := product.New(nsdb)
	prod23.Slug = "SC-6"
	prod23.GetOrCreate("Slug=", prod23.Slug)
	prod23.SetKey("Knc939932KOI")
	prod23.Name = "Scent: Turquoise Bay"
	prod23.Description = "Enjoy a tropical cocktail of island pineapple and coconut combined with blissful basenotes of cedarwood and vanilla"
	prod23.Currency = currency.USD
	prod23.ListPrice = currency.Cents(700)
	prod23.Price = currency.Cents(800)
	prod23.Preorder = true
	prod23.Hidden = false
	prod23.EstimatedDelivery = "Early 2017"
	prod23.Update()

	prod24 := product.New(nsdb)
	prod24.Slug = "SC-6-r"
	prod24.GetOrCreate("Slug=", prod24.Slug)
	prod24.SetKey("Knc93995555I")
	prod24.Name = "Scent: Turquoise Bay (refill)"
	prod24.Description = "Enjoy a tropical cocktail of island pineapple and coconut combined with blissful basenotes of cedarwood and vanilla"
	prod24.Currency = currency.USD
	prod24.ListPrice = currency.Cents(700)
	prod24.Price = currency.Cents(800)
	prod24.Preorder = true
	prod24.Hidden = false
	prod24.EstimatedDelivery = "Early 2017"
	prod24.Update()

	prod25 := product.New(nsdb)
	prod25.Slug = "SC-7"
	prod25.GetOrCreate("Slug=", prod25.Slug)
	prod25.SetKey("Knc9399fnweI")
	prod25.Name = "Scent: White Tea and Ginger"
	prod25.Description = "An intoxicating mixture of white tea notes and pungent, spicy ginger. This exotic mixture is great for every room in the house."
	prod25.Currency = currency.USD
	prod25.ListPrice = currency.Cents(700)
	prod25.Price = currency.Cents(800)
	prod25.Preorder = true
	prod25.Hidden = false
	prod25.EstimatedDelivery = "Early 2017"
	prod25.Update()

	prod26 := product.New(nsdb)
	prod26.Slug = "SC-7-r"
	prod26.GetOrCreate("Slug=", prod26.Slug)
	prod26.SetKey("Knc9399fjfjI")
	prod26.Name = "Scent: White Tea and Ginger (refill)"
	prod26.Description = "An intoxicating mixture of white tea notes and pungent, spicy ginger. This exotic mixture is great for every room in the house."
	prod26.Currency = currency.USD
	prod26.ListPrice = currency.Cents(700)
	prod26.Price = currency.Cents(800)
	prod26.Preorder = true
	prod26.Hidden = false
	prod26.EstimatedDelivery = "Early 2017"
	prod26.Update()

	prod27 := product.New(nsdb)
	prod27.Slug = "SC-8"
	prod27.GetOrCreate("Slug=", prod27.Slug)
	prod27.SetKey("Knc93982fjmI")
	prod27.Name = "Scent: Sheer Linen and Orchid"
	prod27.Description = "A light, refreshing combination lily and orange flowers with lavender and sheer musks."
	prod27.Currency = currency.USD
	prod27.ListPrice = currency.Cents(700)
	prod27.Price = currency.Cents(800)
	prod27.Preorder = true
	prod27.Hidden = false
	prod27.EstimatedDelivery = "Early 2017"
	prod27.Update()

	prod28 := product.New(nsdb)
	prod28.Slug = "SC-8-r"
	prod28.GetOrCreate("Slug=", prod28.Slug)
	prod28.SetKey("Knc9398rrrmI")
	prod28.Name = "Scent: Sheer Linen and Orchid (refill)"
	prod28.Description = "A light, refreshing combination lily and orange flowers with lavender and sheer musks."
	prod28.Currency = currency.USD
	prod28.ListPrice = currency.Cents(700)
	prod28.Price = currency.Cents(800)
	prod28.Preorder = true
	prod28.Hidden = false
	prod28.EstimatedDelivery = "Early 2017"
	prod28.Update()

	prod29 := product.New(nsdb)
	prod29.Slug = "SC-9"
	prod29.GetOrCreate("Slug=", prod29.Slug)
	prod29.SetKey("Knc9jmzzfjmI")
	prod29.Name = "Scent: Coastal Waters"
	prod29.Description = "Fresh ocean breezes gently blowing over a calm beach. Soft white floral background on a mossy musk base. A fresh coastal fragrance."
	prod29.Currency = currency.USD
	prod29.ListPrice = currency.Cents(700)
	prod29.Price = currency.Cents(800)
	prod29.Preorder = true
	prod29.Hidden = false
	prod29.EstimatedDelivery = "Early 2017"
	prod29.Update()

	prod30 := product.New(nsdb)
	prod30.Slug = "SC-9-r"
	prod30.GetOrCreate("Slug=", prod30.Slug)
	prod30.SetKey("Knc9jmlrfjmI")
	prod30.Name = "Scent: Coastal Waters (refill)"
	prod30.Description = "Fresh ocean breezes gently blowing over a calm beach. Soft white floral background on a mossy musk base. A fresh coastal fragrance."
	prod30.Currency = currency.USD
	prod30.ListPrice = currency.Cents(700)
	prod30.Price = currency.Cents(800)
	prod30.Preorder = true
	prod30.Hidden = false
	prod30.EstimatedDelivery = "Early 2017"
	prod30.Update()

	prod31 := product.New(nsdb)
	prod31.Slug = "SC-10"
	prod31.GetOrCreate("Slug=", prod31.Slug)
	prod31.SetKey("Knc9jjfjs4mI")
	prod31.Name = "Scent: Midnight Showers"
	prod31.Description = "A soothing, masculine of bergamot and citrus with delightful hints of sandlewood and oak moss"
	prod31.Currency = currency.USD
	prod31.ListPrice = currency.Cents(700)
	prod31.Price = currency.Cents(800)
	prod31.Preorder = true
	prod31.Hidden = false
	prod31.EstimatedDelivery = "Early 2017"
	prod31.Update()

	prod32 := product.New(nsdb)
	prod32.Slug = "SC-10-r"
	prod32.GetOrCreate("Slug=", prod32.Slug)
	prod32.SetKey("Knc9jfrjs4mI")
	prod32.Name = "Scent: Midnight Showers (refill)"
	prod32.Description = "A soothing, masculine of bergamot and citrus with delightful hints of sandlewood and oak moss"
	prod32.Currency = currency.USD
	prod32.ListPrice = currency.Cents(700)
	prod32.Price = currency.Cents(800)
	prod32.Preorder = true
	prod32.Hidden = false
	prod32.EstimatedDelivery = "Early 2017"
	prod32.Update()
	return []*product.Product{prod1}
})
