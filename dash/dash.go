package platform

import (
	"sort"
	"strconv"

	"appengine/search"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
	"hanzo.io/platform/login"
	"hanzo.io/platform/user"
	"hanzo.io/util/emails"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"
	"hanzo.io/util/router"
	"hanzo.io/util/strings"
	"hanzo.io/util/template"
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

// Defines the routes for the platform
func init() {
	router := router.New("dash")

	loginRequired := middleware.LoginRequired("dash")
	logoutRequired := middleware.LogoutRequired("dash")
	acquireUser := middleware.AcquireUser("dash")
	acquireOrganization := middleware.AcquireOrganization("dash")

	// Frontend
	// router.GET("/", frontend.Index)
	router.GET("/", loginRequired, acquireUser, acquireOrganization, Dashboard)
	// router.GET("/about", frontend.About)
	// router.GET("/contact", frontend.Contact)
	// router.GET("/faq", frontend.Faq)
	// router.GET("/features", frontend.Features)
	// router.GET("/how-it-works", frontend.HowItWorks)
	// router.GET("/pricing", frontend.Pricing)
	// router.GET("/privacy", frontend.Privacy)
	// router.GET("/team", frontend.Team)
	// router.GET("/terms", frontend.Terms)

	// Docs
	// router.GET("/docs", docs.GettingStarted)
	// router.GET("/docs/api", docs.API)
	// router.GET("/docs/checkout", docs.Checkout)
	// router.GET("/docs/hanzo.js", docs.HanzoJS)
	// router.GET("/docs/salesforce", docs.Salesforce)

	// Login
	router.GET("/login", logoutRequired, login.Login)
	router.POST("/login", logoutRequired, login.LoginSubmit)
	router.GET("/logout", login.Logout)

	// Signup
	router.GET("/signup", frontend.Signup)
	// router.GET("/signup", login.Signup)
	// router.POST("/signup", login.SignupSubmit)

	// Password Reset
	// router.GET("/create-password", user.CreatePassword)
	router.GET("/password-reset", login.PasswordReset)
	router.POST("/password-reset", login.PasswordResetSubmit)
	router.GET("/password-reset/:token", login.PasswordResetConfirm)
	router.POST("/password-reset/:token", login.PasswordResetConfirmSubmit)

	// Admin dashboard
	dash := router.Group("")
	dash.Use(loginRequired, acquireUser, acquireOrganization)

	dash.GET("/profile", user.Profile)
	dash.POST("/profile", user.ContactSubmit)
	dash.POST("/profile/password", user.PasswordSubmit)
	dash.GET("/keys", Keys)
	dash.POST("/keys", NewKeys)

	dash.GET("/sendorderconfirmation/:id", SendOrderConfirmation)
	dash.GET("/sendrefundconfirmation/:id", SendRefundConfirmation)
	dash.GET("/sendfulfillmentconfirmation/:id", SendFulfillmentConfirmation)
	dash.POST("/shipwire/ship/:id", ShipOrderUsingShipwire)
	dash.POST("/shipwire/return/:id", ReturnOrderUsingShipwire)

	dash.GET("/organization", Organization)
	dash.POST("/organization", UpdateOrganization)

	dash.GET("/organization/:organizationid/set-active", SetActiveOrganization)

	dash.GET("/settings", user.Profile)

	dash.GET("/search", Search)

	// Stripe connect
	dash.GET("/stripe", Stripe)
	dash.GET("/stripe/callback", StripeCallback)
	dash.GET("/stripe/sync", StripeSync)

	// Salesfoce connect
	dash.GET("/salesforce/callback", SalesforceCallback)
	dash.GET("/salesforce/test", TestSalesforceConnection)
	router.GET("/salesforce/sync", SalesforcePullLatest)
}
