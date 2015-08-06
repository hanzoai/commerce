package admin

import (
	"appengine/search"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
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

func Dashboard(c *gin.Context) {
	Render(c, "backend/index.html")
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
	// Think about rendering a json of all keys after reading in
	// login credentials like Stripe and literally everyone else

	// We use the master key for the dashboard so it is kind of pointless right now

	// We REALLY need a crowdstart domain restricted master key for dashboard
}

func NewKeys(c *gin.Context) {
	org := middleware.GetOrganization(c)

	org.AddDefaultTokens()

	if err := org.Put(); err != nil {
		panic(err)
	}

	c.Writer.WriteHeader(204)
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
