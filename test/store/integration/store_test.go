package store_integration_test

import (
	"testing"
	"time"

	"crowdstart.io/util/gincontext"
	"crowdstart.io/util/log"
	"crowdstart.io/util/task"
	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/httpclient"

	. "crowdstart.io/util/test/ginkgo"

	// Imported so we can call fixtures tasks from here
	_ "crowdstart.io/models/fixtures"
)

func Test(t *testing.T) {
	Setup("store/integration", t)
}

var ctx ae.Context

var _ = BeforeSuite(func() {
	ctx = ae.NewContext(ae.Options{
		Modules:    []string{"default", "store"},
		TaskQueues: []string{"default"},
	})

	// Install product fixtures so we can access store pages
	task.Run(gincontext.New(ctx), "fixtures-products")

	// Wait for fixtures to complete running
	time.Sleep(3 * time.Second)
})

var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("Index", func() {
	It("should be 200 OK", func() {
		client := httpclient.New(ctx, "store")
		res, err := client.Get("/")
		Expect(err).ToNot(HaveOccurred())
		Expect(res.StatusCode).To(Equal(200))
		log.Debug(res.Text())
	})
})
