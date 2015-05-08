package admin

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/order"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
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

// Admin Dashboard
func Dashboard(c *gin.Context) {
	template.Render(c, "admin/dashboard.html")
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
