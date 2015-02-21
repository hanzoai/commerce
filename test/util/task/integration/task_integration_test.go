package task_integration_test

import (
	"path/filepath"
	"testing"
	"time"

	"appengine/memcache"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/zeekay/appenginetesting"

	"crowdstart.io/util/log"
	"crowdstart.io/util/test/httpclient"
)

func Test(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	RegisterFailHandler(Fail)
	RunSpecs(t, "util/task/integration")
}

var (
	ctx *appenginetesting.Context
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	var err error

	//Spin up an appengine dev server with the default module
	ctx, err = appenginetesting.NewContext(&appenginetesting.Options{
		AppId:      "crowdstart-io",
		Debug:      appenginetesting.LogWarning,
		Testing:    GinkgoT(),
		TaskQueues: []string{"default"},
		Modules: []appenginetesting.ModuleConfig{
			{
				Name: "default",
				Path: filepath.Join("../../../../config/test/app.yaml"),
			},
		},
	})

	Expect(err).NotTo(HaveOccurred())
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
