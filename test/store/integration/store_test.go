package store_integration_test

import (
	"testing"
	"time"

	"github.com/headzoo/surf"
	"github.com/headzoo/surf/browser"

	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/task"
	"crowdstart.io/util/test/ae"
	. "crowdstart.io/util/test/ginkgo"
	"crowdstart.io/util/test/httpclient"

	// Imported so we can call fixtures tasks from here
	_ "crowdstart.io/models/fixtures"
)

func Test(t *testing.T) {
	Setup("store/integration", t)
}

var (
	ctx    ae.Context
	client *httpclient.Client
)

var user = struct {
	Email    string
	Password string
}{
	"test@test.com",
	"password",
}

var _ = BeforeSuite(func() {
	ctx = ae.NewContext(ae.Options{
		Modules:    []string{"default", "store"},
		TaskQueues: []string{"default"},
	})

	client = httpclient.New(ctx, "store")

	// Install product fixtures so we can access store pages
	task.Run(gincontext.New(ctx), "fixtures-products")
	task.Run(gincontext.New(ctx), "fixtures-test-users")

	// Wait for fixtures to complete running
	time.Sleep(15 * time.Second)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

func Get200(path string) func() {
	return func() {
		res, err := client.Get(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.StatusCode).To(Equal(200))
	}
}

var _ = Describe("Index", func() {
	It("should be 200 OK", Get200("/"))
})

var _ = Describe("Products", func() {
	It("should be 200 OK", Get200("/products"))
})

func login() *browser.Browser {
	url := client.URL("/login")
	b := surf.NewBrowser()

	if err := b.Open(url); err != nil {
		panic(err)
	}

	loginForm, err := b.Form("form#loginForm")
	if err != nil {
		panic(err)
	}

	loginForm.Input("Email", user.Email)
	loginForm.Input("Password", user.Password)

	err = loginForm.Submit()
	if err != nil {
		panic(err)
	}

	return b
}

var _ = Describe("Login", func() {
	Context("With an existing user", func() {
		It("should redirect to profile.", func() {
			url := client.URL("/login")
			b := surf.NewBrowser()

			err := b.Open(url)
			Expect(err).ToNot(HaveOccurred())

			loginForm, err := b.Form("form#loginForm")
			Expect(err).ToNot(HaveOccurred())

			loginForm.Input("Email", user.Email)
			loginForm.Input("Password", user.Password)
			err = loginForm.Submit()
			Expect(err).ToNot(HaveOccurred())

			errMessage := b.Find("div.errors.error").First().Text()
			Expect(errMessage).To(Equal(""))
		})
	})

	Context("With a nonexistent user", func() {
		It("should error.", func() {
			url := client.URL("/login")
			b := surf.NewBrowser()

			err := b.Open(url)
			Expect(err).ToNot(HaveOccurred())

			loginForm, err := b.Form("form#loginForm")
			Expect(err).ToNot(HaveOccurred())

			err = loginForm.Input("Email", "asjdkas")
			Expect(err).ToNot(HaveOccurred())

			err = loginForm.Input("Password", "asdjkasdj")
			Expect(err).ToNot(HaveOccurred())

			err = loginForm.Submit()
			Expect(err).ToNot(HaveOccurred())

			// Should not redirect.
			Expect(b.Url().String()).To(HaveSuffix("/login"))

			errMessage := b.Find("div.errors.error").First().Text()
			Expect(errMessage).ToNot(Equal(""))
			Expect(errMessage).To(Equal("Invalid email or password"))
		})
	})
})

var _ = Describe("Register", func() {
	Context("With an existing user", func() {
		It("should error", func() {
			// Login and register are both on the same page.
			url := client.URL("/login")
			b := surf.NewBrowser()

			err := b.Open(url)
			Expect(err).ToNot(HaveOccurred())

			form, err := b.Form("form#registerForm")
			Expect(err).ToNot(HaveOccurred())

			err = form.Input("User.FirstName", "John")
			Expect(err).ToNot(HaveOccurred())

			err = form.Input("User.LastName", "Doe")
			Expect(err).ToNot(HaveOccurred())

			err = form.Input("User.Email", "test@test.com")
			Expect(err).ToNot(HaveOccurred())

			err = form.Input("Password", "password")
			Expect(err).ToNot(HaveOccurred())

			err = form.Input("ConfirmPassword", "password")
			Expect(err).ToNot(HaveOccurred())

			err = form.Submit()
			Expect(err).ToNot(HaveOccurred())

			errMessage := b.Find("form#registerForm > div.errors.error").First().Text()
			Expect(errMessage).To(Equal("An account already exists for this email."))
		})
	})
})

var _ = Describe("Profile page", func() {
	It("should render the user's information", func() {
		var b = login()
		err := b.Open(client.URL("/profile"))
		Expect(err).ToNot(HaveOccurred())

		displayedEmail := b.Find("#profileForm > fieldset > div:nth-child(1) > span").Text()
		Expect(displayedEmail).To(Equal(user.Email))
	})
})

var _ = Describe("Create password", func() {
	It("should be 200 OK", Get200("/create-password"))
})

var _ = Describe("Password reset", func() {
	It("should be 200 OK", Get200("/password-reset"))
})
