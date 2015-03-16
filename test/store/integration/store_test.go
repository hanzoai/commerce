package store_integration_test

import (
	"testing"
	"time"

	"github.com/headzoo/surf"

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

var _ = Describe("Login", func() {
	Context("With an existing user", func() {
		It("should redirect to profile.", func() {
			url := client.URL("/login")
			b := surf.NewBrowser()

			err := b.Open(url)
			Expect(err).ToNot(HaveOccurred())

			loginForm, err := b.Form("form#loginForm")
			Expect(err).ToNot(HaveOccurred())

			loginForm.Input("email", user.Email)
			loginForm.Input("password", user.Password)
			err = loginForm.Submit()
			Expect(err).ToNot(HaveOccurred())

			// Expect(b.Url().String()).To(HaveSuffix("/profile"))
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

			loginForm.Input("email", "asjdkas")
			loginForm.Input("password", "asdjkasdj")
			err = loginForm.Submit()
			Expect(err).ToNot(HaveOccurred())

			// Should not redirect.
			Expect(b.Url().String()).To(HaveSuffix("/login"))

			// TODO: Check error message received.
		})
	})
})

var _ = Describe("Create password", func() {
	It("should be 200 OK", Get200("/create-password"))
})

var _ = Describe("Password reset", func() {
	It("should be 200 OK", Get200("/password-reset"))
})
