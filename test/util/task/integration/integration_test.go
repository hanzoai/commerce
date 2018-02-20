package task_integration_test

import (
	"context"
	"net/url"
	"testing"

	"google.golang.org/appengine"
	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/memcache"

	// "hanzo.io/util/test/ae"
	"hanzo.io/log"
	"hanzo.io/util/test/httpclient"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/task/integration", t)
}

var ctx context.Context

// Setup appengine context
var _ = BeforeSuite(func() {
	// ctx, _, _ = aetest.NewContext()
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	// ctx.Close()
})

var _ = Describe("Run", func() {
	FIt("Should call task successfully", func() {
		inst, err := aetest.NewInstance(nil)
		if err != nil {
			log.Fatal(err)
		}

		req, err := inst.NewRequest("GET", "/", nil)
		if err != nil {
			log.Error("Failed to create NewRequest")
			inst.Close()
		}

		log.Warn("DevelopmentServer: %s", appengine.IsDevAppServer())

		ctx := appengine.NewContext(req)

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
