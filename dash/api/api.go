package api

import (
	"sort"
	"strconv"

	"google.golang.org/appengine/search"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
	"hanzo.io/email"
	"hanzo.io/util/json/http"
	"hanzo.io/log"
	"hanzo.io/util/strings"
	"hanzo.io/util/template"
)

// Index
func Index(c *gin.Context) {
	url := config.UrlFor("dash")
	log.Debug("Redirecting to %s", url)
	c.Redirect(302, url)
}

func Dashboard(c *gin.Context) {
	db := datastore.New(c)

	usr := middleware.GetCurrentUser(c)
	var orgNames []*organization.Organization

	if verusEmailRe.MatchString(usr.Email) {
		if _, err := organization.Query(db).Filter("Enabled=", true).GetAll(&orgNames); err != nil {
			log.Warn("Unable to fetch organizations for switcher.", c)
		}
		usr.IsOwner = true
	} else {
		orgIds := usr.Organizations
		for _, orgId := range orgIds {
			org := organization.New(db)
			err := org.GetById(orgId)
			if err != nil {
				log.Error("Could not get Organization with Error %v", err, c)
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

	Render(c, "index.html", "orgNames", orgNames, "orgNumber", len(orgNames))
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
		} else if err != nil {
			break
		}

		u := user.New(db)
		err = datastore.IgnoreFieldMismatch(u.GetById(id))
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

	log.Warn("Get Orders with Query %s", q, c)
	orders := make([]*order.Order, 0)
	for t := index.Search(db.Context, q, nil); ; {
		var doc order.Document
		doc.Init()

		log.Warn("Get Next Order", c)
		id, err := t.Next(&doc)
		if err == search.Done {
			log.Warn("Order Search Done", c)
			break
		} else if err != nil {
			log.Warn("Order Search Error %s", err, c)
			break
		}

		o := order.New(db)
		err = datastore.IgnoreFieldMismatch(o.GetById(id))
		if err != nil {
			log.Warn("Order DB Get Error %s", err, c)
			continue
		}

		log.Warn("Appending Order", c)
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

	email.SendOrderConfirmation(db.Context, org, o, u)

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

	email.SendOrderShipped(db.Context, org, o, u, p)

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
		email.SendOrderRefunded(db.Context, org, o, u, p)
	} else if o.Refunded > 0 {
		email.SendOrderPartiallyRefunded(db.Context, org, o, u, p)
	}

	c.Writer.WriteHeader(204)
}

func Keys(c *gin.Context) {
	// Think about rendering a json of all keys after reading in
	// login credentials like Stripe and literally everyone else

	// We use the master key for the dashboard so it is kind of pointless right now

	// We REALLY need a Hanzo domain restricted master key for dashboard
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
