package store_integration_test

import (
	"testing"
	"time"

	"crowdstart.io/models2/fixtures"
	"crowdstart.io/util/log"
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
		Modules:                []string{"store"},
		PreferAppengineTesting: true,
	})

	client = httpclient.New(ctx, "store")

	// Install product fixtures so we can access store pages
	fixtures.Products(ctx)
	fixtures.TestUsers(ctx)

	// Wait for fixtures to complete running
	time.Sleep(5 * time.Second)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

func Get200(path string) func() {
	return func() {
		res, err := client.Get(path)
		log.Debug(res.Text())
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

var _ = Describe("Create password", func() {
	It("should be 200 OK", Get200("/create-password"))
})

var _ = Describe("Password reset", func() {
	It("should be 200 OK", Get200("/password-reset"))
})
