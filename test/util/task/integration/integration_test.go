package task_integration_test

import (
	"testing"
	"time"

	"appengine/memcache"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/ginkgo"
	"crowdstart.io/util/test/httpclient"
)

func Test(t *testing.T) {
	ginkgo.Setup("util/task/integration", t)
}

var ctx ae.Context

func init() {
	// Setup appengine context
	BeforeSuite(func() {
		ctx = ae.NewContext(ae.Options{
			Modules:    []string{"default"},
			TaskQueues: []string{"default"},
		})

		// Wait for task to run
		time.Sleep(2 * time.Second)
	})

	// Tear-down appengine context
	AfterSuite(func() {
		ctx.Close()
	})

	Describe("Run", func() {
		It("Should call task successfully", func() {
			// Start task
			client := httpclient.New(ctx, "default")

			res, err := client.Get("/task/foo")
			Expect(err).NotTo(HaveOccurred())
			Expect(res.StatusCode).To(Equal(200))

			// Wait for task to run
			time.Sleep(1 * time.Second)

			// Check if memcache is set
			foo, err := memcache.Get(ctx, "foo")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(foo.Value)).To(Equal("bar"))
		})
	})
}
