package admin

import (
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/template"
)

// Index
func Index(c *gin.Context) {
	url := config.UrlFor("platform", "/dashboard")
	log.Debug("Redirecting to %s", url)
	c.Redirect(301, url)
}

type StoreData struct {
	StoreId    string
	StoreName  string
	OrderCount int
	Sales      currency.Cents
}

// Admin Dashboard
func Dashboard(c *gin.Context) {
	// Update Stats
	db := datastore.New(middleware.GetNamespace(c))
	u := user.New(db)

	userCount, err := u.Query().KeysOnly().Count()
	if err != nil {
		panic(err)
	}

	sub := subscriber.New(db)
	subCount, err := sub.Query().KeysOnly().Count()
	if err != nil {
		panic(err)
	}

	o := order.New(db)
	var orders []order.Order
	_, err = o.Query().GetAll(&orders)
	if err != nil {
		panic(err)
	}

	log.Warn("%v", orders)

	var cur currency.Type
	storeDataMap := make(map[string]*StoreData)
	storeDatas := make([]*StoreData, 0)
	for _, ord := range orders {
		if ord.Test && ord.PaymentStatus == payment.Paid {
			continue
		}

		var storeData *StoreData
		var ok bool

		if storeData, ok = storeDataMap[ord.StoreId]; !ok {
			storeData = &StoreData{StoreId: ord.StoreId}
			storeDatas = append(storeDatas, storeData)
			storeDataMap[ord.StoreId] = storeData
		}
		storeData.OrderCount++

		for _, payId := range ord.PaymentIds {
			pay := payment.New(db)
			err = pay.GetById(payId)
			if err != nil {
				panic(err)
			}
			storeData.Sales += pay.AmountTransferred
			cur = pay.CurrencyTransferred
		}
	}

	s := store.New(db)
	var stores []store.Store
	_, err = s.Query().GetAll(&stores)
	if err != nil {
		panic(err)
	}

	for _, stor := range stores {
		if storeData, ok := storeDataMap[stor.Id()]; ok {
			storeData.StoreName = strings.ToUpper(stor.Name)
		}
	}

	orderTotal := 0
	salesTotal := currency.Cents(0)
	for _, storeData := range storeDatas {
		orderTotal += storeData.OrderCount
		salesTotal += storeData.Sales
	}

	template.Render(c, "admin/dashboard.html",
		"userCount", userCount,
		"subCount", subCount,
		"currency", cur,
		"orderTotal", orderTotal,
		"salesTotal", salesTotal,
		"storeDatas", storeDatas,
	)
}

func Products(c *gin.Context) {
	template.Render(c, "admin/list-products.html")
}

func Product(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	p := product.New(db)
	id := c.Params.ByName("id")
	p.MustGet(id)

	template.Render(c, "admin/product.html", "product", p)
}

func Coupons(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	var products []product.Product
	product.Query(db).GetAll(&products)

	template.Render(c, "admin/list-coupons.html", "products", products)
}

func Coupon(c *gin.Context) {
	id := c.Params.ByName("id")
	db := datastore.New(middleware.GetNamespace(c))

	cou := coupon.New(db)
	cou.MustGet(id)

	var products []product.Product
	product.Query(db).GetAll(&products)

	template.Render(c, "admin/coupon.html", "coupon", cou, "products", products)
}

type ProductsMap map[string]product.Product

func (p ProductsMap) Find(id string) product.Product {
	return p[id]
}

func Store(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	s := store.New(db)
	id := c.Params.ByName("id")
	s.MustGet(id)

	listings := make([]store.Listing, 0, len(s.Listings))
	for _, listing := range s.Listings {
		listings = append(listings, listing)
	}

	var products []product.Product
	product.Query(db).GetAll(&products)

	productsMap := make(ProductsMap)
	for _, product := range products {
		productsMap[product.Id()] = product
	}

	template.Render(c, "admin/store.html", "store", s, "listings", listings, "products", products, "productsMap", productsMap)
}

func Stores(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	var products []product.Product
	product.Query(db).GetAll(&products)

	template.Render(c, "admin/list-stores.html", "products", products)
}

func MailingList(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	m := mailinglist.New(db)
	id := c.Params.ByName("id")
	m.MustGet(id)

	template.Render(c, "admin/mailinglist.html", "mailingList", m)
}

func MailingLists(c *gin.Context) {
	template.Render(c, "admin/list-mailinglists.html")
}

func Order(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("id")
	o.MustGet(id)

	u := user.New(db)
	u.MustGet(o.UserId)

	template.Render(c, "admin/order.html", "order", o, "user", u)
}

func Orders(c *gin.Context) {
	template.Render(c, "admin/list-orders.html")
}

func Organization(c *gin.Context) {
	template.Render(c, "admin/organization.html")
}

func Keys(c *gin.Context) {
	template.Render(c, "admin/keys.html")
}

func NewKeys(c *gin.Context) {
	org := middleware.GetOrganization(c)

	org.ClearTokens()
	org.AddToken("live-secret-key", permission.Admin)
	org.AddToken("live-published-key", permission.Published)
	org.AddToken("test-secret-key", permission.Admin)
	org.AddToken("test-published-key", permission.Published)

	if err := org.Put(); err != nil {
		panic(err)
	}

	template.Render(c, "admin/keys.html")
}
