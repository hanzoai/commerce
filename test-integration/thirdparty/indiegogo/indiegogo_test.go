package test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/hanzoai/commerce/thirdparty/indiegogo"
	"github.com/hanzoai/commerce/log"
)

func Test(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	RegisterFailHandler(Fail)
	RunSpecs(t, "thirdparty/indiegogo")
}

var _ = Describe("indiegogo", func() {
	Context("NewRow", func() {
		It("should parse Indiegogo records into our row representation", func() {
			r := indiegogo.NewRow([]string{
				`7R66kcGb9O`,
				`2267279`,
				`7842535`,
				`Unfulfilled`,
				`2014-08-22 08:55:14 -0700`,
				``,
				`Visible`,
				`Michael l Coleman`,
				`dev@hanzo.ai`,
				`$499`,
				`$499 Now and $949 Due at Ship`,
				`Mike Coleman `,
				`4960 oarsman way`,
				`""`,
				`Sarasota`,
				`Florida`,
				`"=""34243"""`,
				`United States`,
			})

			Expect(r.FirstName).To(Equal("Mike"))
			Expect(r.LastName).To(Equal("Coleman"))
		})
	})
})
