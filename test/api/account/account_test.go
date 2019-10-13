package test

import (
	"net/http"
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/models/transaction"
	"hanzo.io/models/transaction/util"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/bitcoin"
	"hanzo.io/thirdparty/ethereum"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginclient"
	. "hanzo.io/util/test/ginkgo"

	accountApi "hanzo.io/api/account"
)

func Test(t *testing.T) {
	Setup("api/account", t)
}

var (
	ctx         ae.Context
	cl          *Client
	accessToken string
	db          *datastore.Datastore
	org         *organization.Organization
	u           *user.User
	usr         *user.User
)

// Setup appengine context
var _ = BeforeSuite(func() {
	// Create a new app engine context
	ctx = ae.NewContext()

	// Create mock gin context that we can use with fixtures
	c := gincontext.New(ctx)

	// Run fixtures
	u = fixtures.User(c).(*user.User)
	org = fixtures.Organization(c).(*organization.Organization)

	// Setup client and add routes for account API tests.
	cl = New(ctx)
	accountApi.Route(cl.Router)

	// Create organization for tests, accessToken
	tok, _ := org.GetTokenByName("test-published-key")
	accessToken = tok.String

	err := org.Put()
	Expect(err).NotTo(HaveOccurred())

	// Set authorization header for subsequent requests
	cl.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken)
	})

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))
	usr = user.New(db)
	usr.Username = "redranger"
	usr.Email = "dev@hanzo.ai"
	usr.SetPassword("Z0rd0N")
	usr.Enabled = true
	usr.MustPut()

	usr2 := user.New(db)
	usr2.Email = "dev@hanzo.ai"
	usr2.SetPassword("ilikedragons")
	usr2.Enabled = false
	usr2.MustPut()

	usr3 := user.New(db)
	usr3.FirstName = "Z"
	usr3.LastName = "T"
	usr3.Email = "zack@taylor.edu"
	usr3.Enabled = false
	usr3.MustPut()

	usr4 := user.New(db)
	usr4.FirstName = "Z"
	usr4.LastName = "T"
	usr4.Email = "dev@hanzo.ai"
	usr4.SetPassword("blackisthenewred")
	usr4.Enabled = true
	usr4.MustPut()

	usr5 := user.New(db)
	usr5.FirstName = "Z"
	usr5.LastName = "T"
	usr5.Username = "zack"
	usr5.Email = "dev@hanzo.ai"
	usr5.SetPassword("blackisthenewred")
	usr5.Enabled = true
	usr5.MustPut()

	ethereum.Test(true)
	bitcoin.Test(true)
	cl.IgnoreErrors(true)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

type createRes struct {
	user.User

	Token string `json:"token"`
}

type loginRes struct {
	Token string `json:"token"`
}

type withdrawRes struct {
	TransactionId string `json:"transactionId"`
}

var _ = Describe("account", func() {
	Context("Create without Username", func() {
		It("Should fail without token", func() {
			at := accessToken
			accessToken = "123"

			req := `{
				"firstName": "Zack",
				"lastName": "Taylor",
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N",
				"passwordConfirm": "Z0rd0N"
			}`

			res := createRes{}

			log.Debug("Response %s", cl.Post("/account/create", req, &res, 401))
			accessToken = at
		})
	})
	Context("Create without Username", func() {
		It("Should create an account", func() {
			req := `{
				"firstName": "Zack",
				"lastName": "Taylor",
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N",
				"passwordConfirm": "Z0rd0N"
			}`

			res := createRes{}

			log.Debug("Response %s", cl.Post("/account/create", req, &res))
			Expect(res.User.FirstName).To(Equal("Zack"))
			Expect(res.User.LastName).To(Equal("Taylor"))
			Expect(res.User.Email).To(Equal("dev@hanzo.ai"))
		})

		It("Should create an account if it already exists but has no password", func() {
			req := `{
				"firstName": "Zack",
				"lastName": "Taylor",
				"email": "zack@taylor.edu",
				"password": "Z0rd0N",
				"passwordConfirm": "Z0rd0N"
			}`

			res := createRes{}

			log.Debug("Response %s", cl.Post("/account/create", req, &res))
			Expect(res.FirstName).To(Equal("Zack"))
			Expect(res.LastName).To(Equal("Taylor"))
			Expect(res.Email).To(Equal("zack@taylor.edu"))
		})

		It("Should create not create account if it already exists and has a password", func() {
			req := `{
				"firstName": "Zack",
				"lastName": "Taylor",
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N",
				"passwordConfirm": "Z0rd0N"
			}`

			res := createRes{}

			log.Debug("Response %s", cl.Post("/account/create", req, &res, 400))
		})

		It("Should fail if org requires username", func() {
			org.SignUpOptions.UsernameRequired = true
			org.MustUpdate()

			req := `{
				"firstName": "Zack",
				"lastName": "Taylor",
				"email": "zack@taylor.ninja",
				"password": "Z0rd0N",
				"passwordConfirm": "Z0rd0N"
			}`

			res := createRes{}

			log.Debug("Response %s", cl.Post("/account/create", req, &res, 400))

			org.SignUpOptions.UsernameRequired = false
			org.MustUpdate()
		})
	})

	Context("Create with Username", func() {
		It("Should create an account", func() {
			req := `{
				"username": "ZackShouldCreateAccount",
				"firstName": "Zack",
				"lastName": "Taylor",
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N",
				"passwordConfirm": "Z0rd0N"
			}`

			res := createRes{}

			log.Debug("Response %s", cl.Post("/account/create", req, &res))
			Expect(res.User.FirstName).To(Equal("Zack"))
			Expect(res.User.LastName).To(Equal("Taylor"))
			Expect(res.User.Email).To(Equal("dev@hanzo.ai"))
			Expect(res.User.Username).To(Equal("zackshouldcreateaccount"))
		})

		It("Should create an account if it already exists but has no password", func() {
			req := `{
				"username": "ZackNoPass",
				"firstName": "Zack",
				"lastName": "Taylor",
				"email": "zack2@taylor.edu",
				"password": "Z0rd0N",
				"passwordConfirm": "Z0rd0N"
			}`

			res := createRes{}

			log.Debug("Response %s", cl.Post("/account/create", req, &res))
			Expect(res.FirstName).To(Equal("Zack"))
			Expect(res.LastName).To(Equal("Taylor"))
			Expect(res.Email).To(Equal("zack2@taylor.edu"))
			Expect(res.User.Username).To(Equal("zacknopass"))
		})

		It("Should create not create account if it already exists and has a password", func() {
			req := `{
				"username": "Zack",
				"firstName": "Zack",
				"lastName": "Taylor",
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N",
				"passwordConfirm": "Z0rd0N"
			}`

			res := createRes{}

			log.Debug("Response %s", cl.Post("/account/create", req, &res, 400))
		})

		It("Should create not create account if the username already exists", func() {
			req := `{
				"username": "Zack",
				"firstName": "Zack",
				"lastName": "Taylor",
				"email": "zack2@taylor.gov",
				"password": "Z0rd0N",
				"passwordConfirm": "Z0rd0N"
			}`

			res := createRes{}

			log.Debug("Response %s", cl.Post("/account/create", req, &res, 400))
		})
	})

	Context("Login", func() {
		It("Should allow login with email", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N"
			}`

			res := loginRes{}

			cl.Post("/account/login", req, &res)
			Expect(res.Token).ToNot(Equal(""))
		})

		It("Should allow login with username", func() {
			req := `{
				"username": "redranger",
				"password": "Z0rd0N"
			}`

			res := loginRes{}

			cl.Post("/account/login", req, &res)
			Expect(res.Token).ToNot(Equal(""))
		})

		It("Should disallow login with disabled account", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "ilikedragon"
			}`

			cl.Post("/account/login", req, nil, 401)
		})

		It("Should disallow login with wrong password", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "z3d"
			}`

			cl.Post("/account/login", req, nil, 401)
		})

		It("Should disallow login with wrong email", func() {
			req := `{
				"email": "billy@blue.co.uk",
				"password": "bloo"
			}`

			cl.Post("/account/login", req, nil, 401)
		})

	})

	// Reenable when we care about crypto again
	XContext("Withdraw", func() {
		It("Should withdraw ethereum", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N"
			}`

			res := loginRes{}

			cl.Post("/account/login", req, &res)

			oat := accessToken
			accessToken = res.Token

			// Deposit
			tr1 := transaction.New(db)
			tr1.DestinationId = usr.Id()
			tr1.DestinationKind = "user"
			tr1.Currency = currency.ETH
			tr1.Amount = currency.Cents(100)
			tr1.Type = transaction.Deposit
			tr1.Test = true
			tr1.MustCreate()

			req2 := `{
				"to": "0x0",
				"name": "Test Ethereum",
				"amount": 100,
				"fee": 0
			}`

			res2 := withdrawRes{}

			cl.Post("/account/withdraw", req2, &res2)

			Expect(res2.TransactionId).To(Equal("0x0"))

			datas, err := util.GetTransactionsByCurrency(db.Context, usr.Id(), "user", currency.ETH, true)
			Expect(err).ToNot(HaveOccurred())

			data := datas.Data[currency.ETH]
			Expect(data.Balance - data.Holds).To(Equal(currency.Cents(0)))

			accessToken = oat
		})

		It("Shouldn't withdraw held ethereum", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N"
			}`

			res := loginRes{}

			cl.Post("/account/login", req, &res)

			oat := accessToken
			accessToken = res.Token

			// Deposit
			tr1 := transaction.New(db)
			tr1.DestinationId = usr.Id()
			tr1.DestinationKind = "user"
			tr1.Currency = currency.ETH
			tr1.Amount = currency.Cents(100)
			tr1.Type = transaction.Deposit
			tr1.Test = true
			tr1.MustCreate()

			tr2 := transaction.New(db)
			tr2.SourceId = usr.Id()
			tr2.SourceKind = "user"
			tr2.Currency = currency.ETH
			tr2.Amount = currency.Cents(100)
			tr2.Type = transaction.Hold
			tr2.Test = true
			tr2.MustCreate()

			req2 := `{
				"to": "0x0",
				"name": "Test Ethereum",
				"amount": 100,
				"fee": 0
			}`

			res2 := &ApiError{}

			cl.Post("/account/withdraw", req2, &res2)

			Expect(res2.Error.Type).To(Equal("api-error"))
			Expect(res2.Error.Message).To(Equal("Source has insufficient funds"))

			accessToken = oat
		})

		It("Shouldn't withdraw held withdrawn ethereum", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N"
			}`

			res := loginRes{}

			cl.Post("/account/login", req, &res)

			oat := accessToken
			accessToken = res.Token

			// Deposit
			tr1 := transaction.New(db)
			tr1.DestinationId = usr.Id()
			tr1.DestinationKind = "user"
			tr1.Currency = currency.ETH
			tr1.Amount = currency.Cents(100)
			tr1.Type = transaction.Deposit
			tr1.Test = true
			tr1.MustCreate()

			tr2 := transaction.New(db)
			tr2.SourceId = usr.Id()
			tr2.SourceKind = "user"
			tr2.Currency = currency.ETH
			tr2.Amount = currency.Cents(100)
			tr2.Type = transaction.Withdraw
			tr2.Test = true
			tr2.MustCreate()

			req2 := `{
				"to": "0x0",
				"name": "Test Ethereum",
				"amount": 100,
				"fee": 0
			}`

			res2 := &ApiError{}

			cl.Post("/account/withdraw", req2, &res2)

			Expect(res2.Error.Type).To(Equal("api-error"))
			Expect(res2.Error.Message).To(Equal("Source has insufficient funds"))

			accessToken = oat
		})

		It("Should withdraw bitcoin", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N"
			}`

			res := loginRes{}

			cl.Post("/account/login", req, &res)

			oat := accessToken
			accessToken = res.Token

			// Deposit
			tr1 := transaction.New(db)
			tr1.DestinationId = usr.Id()
			tr1.DestinationKind = "user"
			tr1.Currency = currency.BTC
			tr1.Amount = currency.Cents(100)
			tr1.Type = transaction.Deposit
			tr1.Test = true
			tr1.MustCreate()

			req2 := `{
				"to": "mwazxpfoUPnVXjBLqTMPvVESuKgonxySBU",
				"name": "Test Bitcoin",
				"amount": 100,
				"fee": 0
			}`

			res2 := withdrawRes{}

			cl.Post("/account/withdraw", req2, &res2)

			Expect(res2.TransactionId).To(Equal("0"))

			datas, err := util.GetTransactionsByCurrency(db.Context, usr.Id(), "user", currency.BTC, true)
			Expect(err).ToNot(HaveOccurred())

			data := datas.Data[currency.BTC]
			Expect(data.Balance - data.Holds).To(Equal(currency.Cents(0)))

			accessToken = oat
		})

		It("Shouldn't withdraw held bitcoin", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N"
			}`

			res := loginRes{}

			cl.Post("/account/login", req, &res)

			oat := accessToken
			accessToken = res.Token

			// Deposit
			tr1 := transaction.New(db)
			tr1.DestinationId = usr.Id()
			tr1.DestinationKind = "user"
			tr1.Currency = currency.BTC
			tr1.Amount = currency.Cents(100)
			tr1.Type = transaction.Deposit
			tr1.Test = true
			tr1.MustCreate()

			tr2 := transaction.New(db)
			tr2.SourceId = usr.Id()
			tr2.SourceKind = "user"
			tr2.Currency = currency.BTC
			tr2.Amount = currency.Cents(100)
			tr2.Type = transaction.Hold
			tr2.Test = true
			tr2.MustCreate()

			req2 := `{
				"to": "0x0",
				"name": "Test Bitcoin",
				"amount": 100,
				"fee": 0
			}`

			res2 := &ApiError{}

			cl.Post("/account/withdraw", req2, &res2)

			Expect(res2.Error.Type).To(Equal("api-error"))
			Expect(res2.Error.Message).To(Equal("Source has insufficient funds"))

			accessToken = oat
		})

		It("Shouldn't withdraw held withdrawn bitcoin", func() {
			req := `{
				"email": "dev@hanzo.ai",
				"password": "Z0rd0N"
			}`

			res := loginRes{}

			cl.Post("/account/login", req, &res)

			oat := accessToken
			accessToken = res.Token

			// Deposit
			tr1 := transaction.New(db)
			tr1.DestinationId = usr.Id()
			tr1.DestinationKind = "user"
			tr1.Currency = currency.BTC
			tr1.Amount = currency.Cents(100)
			tr1.Type = transaction.Deposit
			tr1.Test = true
			tr1.MustCreate()

			tr2 := transaction.New(db)
			tr2.SourceId = usr.Id()
			tr2.SourceKind = "user"
			tr2.Currency = currency.BTC
			tr2.Amount = currency.Cents(100)
			tr2.Type = transaction.Withdraw
			tr2.Test = true
			tr2.MustCreate()

			req2 := `{
				"to": "0x0",
				"name": "Test Bitcoin",
				"amount": 100,
				"fee": 0
			}`

			res2 := &ApiError{}

			cl.Post("/account/withdraw", req2, &res2)

			Expect(res2.Error.Type).To(Equal("api-error"))
			Expect(res2.Error.Message).To(Equal("Source has insufficient funds"))

			accessToken = oat
		})
	})
})
