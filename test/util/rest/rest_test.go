package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"crowdstart.io/util/rest"
	"crowdstart.io/util/test/ae"

	. "crowdstart.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/rest", t)
}

var ctx ae.Context

// Setup appengine context
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

type Model struct {
	Name string
}

func (m Model) Kind() string {
	return "test-model"
}

func newRouter() *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("appengine", ctx)
	})
	return router
}

func request(router *gin.Engine, method, url string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

var _ = Describe("New", func() {
	It("Should create a new Rest object with CRUD routes", func() {
		router := newRouter()

		// Create routes for Model
		rest := rest.New(Model{})
		rest.Route(router)

		w := request(router, "GET", "/test-model")
		Expect(w.Code).To(Equal(200))
	})
})
