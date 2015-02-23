package test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"appengine/aetest"
	"appengine/urlfetch"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	// "github.com/zeekay/aetest"

	"crowdstart.io/models/fixtures"
	"crowdstart.io/store"
	"crowdstart.io/util/log"
)

func TestStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Store testing suite")
}

var (
	instance aetest.Instance
	client   *http.Client
	ctx      aetest.Context
	server   *httptest.Server
)

var _ = BeforeSuite(func() {
	opts := &aetest.Options{StronglyConsistentDatastore: true}
	var err error
	ctx, err = aetest.NewContext(opts)
	Expect(err).ToNot(HaveOccurred())

	aetest.PrepareDevAppserver = func() error {
		log.Info("Preparing app server")
		fixtures.Install.Call(ctx, "all")
		return nil
	}

	client = urlfetch.Client(ctx)

	instance, err = aetest.NewInstance(opts)
	Expect(err).ToNot(HaveOccurred())

	server = httptest.NewServer(store.Engine)
})

var _ = AfterSuite(func() {
	err := instance.Close()
	Expect(err).ToNot(HaveOccurred())

	err = ctx.Close()
	Expect(err).ToNot(HaveOccurred())

	server.Close()
})

const (
	GET = "GET"
)

var _ = Describe("Index", func() {
	It("should be 200 OK", func() {
		req, err := http.NewRequest(GET, server.URL, nil)
		Expect(err).ToNot(HaveOccurred())

		res, err := client.Do(req)
		body, _ := ioutil.ReadAll(res.Body)
		println("---")
		println(string(body))
		println("---")
		Expect(err).ToNot(HaveOccurred())

		Expect(res.StatusCode).To(Equal(http.StatusOK))
	})
})
