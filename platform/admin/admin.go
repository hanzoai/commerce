package admin

import (
	"time"

	"appengine/memcache"
	"appengine/search"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
	"crowdstart.com/models/subscriber"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/user"
	"crowdstart.com/util/emails"
	"crowdstart.com/util/log"
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

type IRef struct {
	I int
}

type ICCSRef struct {
	I  int
	C  currency.Cents
	C2 currency.Type
	S  []*StoreData
}

// Admin Dashboard
func Dashboard(c *gin.Context) {
	// Update Stats
	db := datastore.New(middleware.GetNamespace(c))
	u := user.New(db)

	orgName := middleware.GetOrganization(c).Name

	ctx := db.Context
	key := orgName + "-userCount"
	ir := IRef{}

	userCount := 0
	subCount := 0
	orderTotal := 0
	salesTotal := currency.Cents(0)

	_, err := memcache.Gob.Get(ctx, key, &ir)
	if err != nil {
		userCount, err = u.Query().KeysOnly().Count()
		if err != nil {
			panic(err)
		}

		item := &memcache.Item{
			Key:        key,
			Object:     IRef{userCount},
			Expiration: time.Duration(time.Minute * 17),
		}

		memcache.Gob.Set(db.Context, item)
	} else {
		userCount = ir.I
	}

	key = orgName + "-subCount"

	_, err = memcache.Gob.Get(ctx, key, &ir)
	if err != nil {
		sub := subscriber.New(db)
		subCount, err = sub.Query().KeysOnly().Count()
		if err != nil {
			panic(err)
		}

		item := &memcache.Item{
			Key:        key,
			Object:     IRef{subCount},
			Expiration: time.Duration(time.Minute * 19),
		}

		memcache.Gob.Set(db.Context, item)
	} else {
		subCount = ir.I
	}

	key = orgName + "-totalCount"
	// iccsr := ICCSRef{}
	storeDatas := make([]*StoreData, 0)
	var cur currency.Type

	// _, err = memcache.Gob.Get(ctx, key, &iccsr)
	// if err != nil {
	// 	o := order.New(db)
	// 	var orders []order.Order
	// 	_, err = o.Query().GetAll(&orders)
	// 	if err != nil {
	// 		panic(err)
	// 	}

	// 	log.Warn("%v", orders)

	// 	storeDataMap := make(map[string]*StoreData)
	// 	for _, ord := range orders {
	// 		if ord.Test && ord.PaymentStatus == payment.Paid {
	// 			continue
	// 		}

	// 		var storeData *StoreData
	// 		var ok bool

	// 		if storeData, ok = storeDataMap[ord.StoreId]; !ok {
	// 			storeData = &StoreData{StoreId: ord.StoreId}
	// 			storeDatas = append(storeDatas, storeData)
	// 			storeDataMap[ord.StoreId] = storeData
	// 		}
	// 		storeData.OrderCount++

	// 		for _, payId := range ord.PaymentIds {
	// 			pay := payment.New(db)
	// 			err = pay.GetById(payId)
	// 			if err != nil {
	// 				continue
	// 			}
	// 			if pay.AmountTransferred == 0 {
	// 				storeData.Sales += pay.AmountTransferred
	// 			} else {
	// 				storeData.Sales += pay.Amount
	// 			}

	// 			if pay.CurrencyTransferred == "" {
	// 				cur = pay.CurrencyTransferred
	// 			} else {
	// 				cur = pay.Currency
	// 			}
	// 		}
	// 	}

	// 	s := store.New(db)
	// 	var stores []store.Store
	// 	_, err = s.Query().GetAll(&stores)
	// 	if err != nil {
	// 		panic(err)
	// 	}

	// 	for _, stor := range stores {
	// 		if storeData, ok := storeDataMap[stor.Id()]; ok {
	// 			storeData.StoreName = strings.ToUpper(stor.Name)
	// 		}
	// 	}

	// 	orderTotal = 0
	// 	salesTotal = currency.Cents(0)
	// 	for _, storeData := range storeDatas {
	// 		orderTotal += storeData.OrderCount
	// 		salesTotal += storeData.Sales
	// 	}

	// 	item := &memcache.Item{
	// 		Key:        key,
	// 		Object:     ICCSRef{orderTotal, salesTotal, cur, storeDatas},
	// 		Expiration: time.Duration(time.Minute * 23),
	// 	}

	// 	memcache.Gob.Set(db.Context, item)
	// } else {
	// 	orderTotal = iccsr.I
	// 	salesTotal = iccsr.C
	// 	cur = iccsr.C2
	// 	storeDatas = iccsr.S
	// }

	log.Warn("%v %v %v %v %v", userCount, subCount, cur, orderTotal, salesTotal)

	Render(c, "admin/dashboard.html",
		"userCount", userCount,
		"subCount", subCount,
		"currency", cur,
		"orderTotal", orderTotal,
		"salesTotal", salesTotal,
		"storeDatas", storeDatas,
	)
}

func Search(c *gin.Context) {
	q := c.Request.URL.Query().Get("q")

	u := user.User{}
	index, err := search.Open(u.Kind())
	if err != nil {
		return
	}

	db := datastore.New(middleware.GetNamespace(c))

	users := make([]*user.User, 0)
	for t := index.Search(db.Context, q, nil); ; {
		var doc user.Document
		id, err := t.Next(&doc)
		if err == search.Done {
			break
		}
		if err != nil {
			break
		}

		u := user.New(db)
		err = u.GetById(id)
		if err != nil {
			continue
		}

		users = append(users, u)
	}

	o := order.Order{}
	index, err = search.Open(o.Kind())
	if err != nil {
		return
	}

	orders := make([]*order.Order, 0)
	for t := index.Search(db.Context, q, nil); ; {
		var doc order.Document
		id, err := t.Next(&doc)
		if err == search.Done {
			break
		}
		if err != nil {
			break
		}

		o := order.New(db)
		err = o.GetById(id)
		if err != nil {
			continue
		}

		orders = append(orders, o)
	}

	Render(c, "admin/search-results.html", "users", users, "orders", orders)
}

func Products(c *gin.Context) {
	Render(c, "admin/list-products.html")
}

func Product(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	p := product.New(db)
	id := c.Params.ByName("id")
	p.MustGet(id)

	Render(c, "admin/product.html", "product", p)
}

func Coupons(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	var products []product.Product
	product.Query(db).GetAll(&products)

	Render(c, "admin/list-coupons.html", "products", products)
}

func Coupon(c *gin.Context) {
	id := c.Params.ByName("id")
	db := datastore.New(middleware.GetNamespace(c))

	cou := coupon.New(db)
	cou.MustGet(id)

	var products []product.Product
	product.Query(db).GetAll(&products)

	Render(c, "admin/coupon.html", "coupon", cou, "products", products)
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

	Render(c, "admin/store.html", "store", s, "listings", listings, "products", products, "productsMap", productsMap)
}

func Stores(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	var products []product.Product
	product.Query(db).GetAll(&products)

	Render(c, "admin/list-stores.html", "products", products)
}

func MailingList(c *gin.Context) {
	db := datastore.New(middleware.GetNamespace(c))

	m := mailinglist.New(db)
	id := c.Params.ByName("id")
	m.MustGet(id)

	Render(c, "admin/mailinglist.html", "mailingList", m)
}

func MailingLists(c *gin.Context) {
	Render(c, "admin/list-mailinglists.html")
}

func User(c *gin.Context) {
	Render(c, "admin/user.html")
}

func Users(c *gin.Context) {
	Render(c, "admin/list-users.html")
}

func Order(c *gin.Context) {
	Render(c, "admin/order.html")
}

func Orders(c *gin.Context) {
	Render(c, "admin/list-orders.html")
}

func SendOrderConfirmation(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("id")
	o.MustGet(id)

	u := user.New(db)
	u.MustGet(o.UserId)

	emails.SendOrderConfirmationEmail(c, org, o, u)

	Render(c, "admin/order.html")
}

func Organization(c *gin.Context) {
	Render(c, "admin/organization.html")
}

func Keys(c *gin.Context) {
	Render(c, "admin/keys.html")
}

func NewKeys(c *gin.Context) {
	org := middleware.GetOrganization(c)

	org.AddDefaultTokens()

	if err := org.Put(); err != nil {
		panic(err)
	}

	Render(c, "admin/keys.html")
}

func Render(c *gin.Context, name string, args ...interface{}) {
	db := datastore.New(c)
	org := organization.New(db)
	if err := org.GetById("crowdstart"); err == nil {
		args = append(args, "crowdstartId", org.Id())
	} else {
		args = append(args, "crowdstartId", "")
	}
	log.Warn("Z%s", org.Id())

	template.Render(c, name, args...)
}
