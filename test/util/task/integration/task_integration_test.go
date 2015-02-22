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
	ginkgo.SetupTest("util/task/integration", t)
}

var (
	ctx ae.Context
)

// Setup appengine context
var _ = BeforeSuite(func() {
	_ctx, err := ae.NewContext(ae.Options{
		Modules:    []string{"default"},
		TaskQueues: []string{"default"},
	})
	Expect(err).NotTo(HaveOccurred())

	// Save reference to appengine.Context
	ctx = _ctx

	// Wait for task to run
	time.Sleep(5 * time.Second)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("Run", func() {
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
