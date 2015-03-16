package test

import (
	"testing"
	"time"

	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/httpclient"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/rest", t)
}

var ctx ae.Context

// Setup appengine context
var _ = BeforeSuite(func() {
	ctx = ae.NewContext(ae.Options{
		Modules:                []string{"api"},
		PreferAppengineTesting: true,
	})

	// Wait for task to run
	time.Sleep(3 * time.Second)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("Restify", func() {
	It("Should add add RESTful routes for given model", func() {
		client := httpclient.New(ctx, "api")

		res, err := client.Get("/token2")
		Expect(err).NotTo(HaveOccurred())
		Expect(res.StatusCode).To(Equal(200))
	})
})
