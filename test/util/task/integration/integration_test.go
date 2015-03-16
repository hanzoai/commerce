package task_integration_test

import (
	"net/url"
	"testing"
	"time"

	"appengine/memcache"

	"crowdstart.io/util/test/ae"
	"crowdstart.io/util/test/httpclient"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/task/integration", t)
}

var ctx ae.Context

// Setup appengine context
var _ = BeforeSuite(func() {
	ctx = ae.NewContext(ae.Options{
		Modules:    []string{"default"},
		TaskQueues: []string{"default"},
	})

	// Wait for task to run
	time.Sleep(3 * time.Second)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("Run", func() {
	It("Should call task successfully", func() {
		// Start task
		client := httpclient.New(ctx, "default")

		res, err := client.PostForm("/task/foo", url.Values{})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.StatusCode).To(Equal(200))

		// Wait for task to run
		time.Sleep(2 * time.Second)

		// Check if memcache is set
		foo, err := memcache.Get(ctx, "foo")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(foo.Value)).To(Equal("bar"))
	})

	It("Should call nested tasks successfully", func() {
		// Start task
		client := httpclient.New(ctx, "default")

		res, err := client.PostForm("/task/nested-baz", url.Values{})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.StatusCode).To(Equal(200))

		// Wait for task to run
		time.Sleep(8 * time.Second)

		// Check if memcache is set
		baz, err := memcache.Get(ctx, "baz")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(baz.Value)).To(Equal("qux"))
	})
})
