package admin

import (
	"sort"
	"strconv"

	"appengine/search"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
	"crowdstart.com/util/emails"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
	"crowdstart.com/util/strings"
	"crowdstart.com/util/template"
)

// Index
func Index(c *gin.Context) {
	url := config.UrlFor("platform")
	log.Debug("Redirecting to %s", url)
	c.Redirect(302, url)
}

func Dashboard(c *gin.Context) {
	db := datastore.New(c)

	usr := middleware.GetCurrentUser(c)
	var orgNames []*organization.Organization

	if verusEmailRe.MatchString(usr.Email) {
		if _, err := organization.Query(db).GetAll(&orgNames); err != nil {
			log.Warn("Unable to fetch organizations for switcher.")
		}

		usr.IsOwner = true
	} else {
		orgIds := usr.Organizations
		for _, orgId := range orgIds {
			org := organization.New(db)
			err := org.GetById(orgId)
			if err != nil {
				continue
			}
			orgNames = append(orgNames, org)
		}

		org := middleware.GetOrganization(c)

		for _, userId := range org.Owners {
			if userId == usr.Id() {
				usr.IsOwner = true
				break
			}
		}
	}

	// Sort organizations by name
	sort.Sort(organization.ByName(orgNames))

	Render(c, "backend/index.html", "orgNames", orgNames, "orgNumber", len(orgNames))
}

type SearchResults struct {
	Users  []*user.User   `json:"users"`
	Orders []*order.Order `json:"orders"`
}

func Search(c *gin.Context) {
	q := c.Request.URL.Query().Get("q")

	// Detect order numbers
	if _, err := strconv.Atoi(strings.StripWhitespace(q)); err == nil {
		q = "Number:" + q
	}

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

	http.Render(c, 200, SearchResults{users, orders})
}

func SendOrderConfirmation(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("id")
	o.MustGetById(id)

	u := user.New(db)
	u.MustGetById(o.UserId)

	emails.SendOrderConfirmationEmail(db.Context, org, o, u)

	c.Writer.WriteHeader(204)
}

func SendFulfillmentConfirmation(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("id")
	o.MustGetById(id)

	u := user.New(db)
	u.MustGetById(o.UserId)

	p := payment.New(db)
	p.MustGetById(o.PaymentIds[0])

	emails.SendFulfillmentEmail(db.Context, org, o, u, p)

	c.Writer.WriteHeader(204)
}

func SendRefundConfirmation(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(middleware.GetNamespace(c))

	o := order.New(db)
	id := c.Params.ByName("id")
	o.MustGetById(id)

	u := user.New(db)
	u.MustGetById(o.UserId)

	p := payment.New(db)
	p.MustGetById(o.PaymentIds[0])

	if o.Refunded == o.Paid {
		emails.SendFullRefundEmail(db.Context, org, o, u, p)
	} else if o.Refunded > 0 {
		emails.SendPartialRefundEmail(db.Context, org, o, u, p)
	}

	c.Writer.WriteHeader(204)
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
