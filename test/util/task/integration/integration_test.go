package task_integration_test

import (
	"net/url"
	"testing"

	"google.golang.org/appengine/memcache"

	"hanzo.io/util/test/ae"
	"hanzo.io/util/test/httpclient"

	. "hanzo.io/util/test/ginkgo"
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
		Noisy:      true,
	})
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

		// Try and get value from memcache 5 times before giving up
		var foo *memcache.Item
		Retry(5, func() error {
			foo, err = memcache.Get(ctx, "foo")
			return err
		})

		// Check if memcache is set
		Expect(err).NotTo(HaveOccurred())
		Expect(string(foo.Value)).To(Equal("bar"))
	})

	It("Should call nested tasks successfully", func() {
		// Start task
		client := httpclient.New(ctx, "default")

		res, err := client.PostForm("/task/nested-baz", url.Values{})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.StatusCode).To(Equal(200))

		// Try to get value from memcache
		var baz *memcache.Item
		Retry(5, func() error {
			baz, err = memcache.Get(ctx, "baz")
			return err
		})

		// Check if memcache is set
		Expect(err).NotTo(HaveOccurred())
		Expect(string(baz.Value)).To(Equal("qux"))
	})
})
