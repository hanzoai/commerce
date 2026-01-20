package test

import (
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/ads/adcampaign"
	"github.com/hanzoai/commerce/models/ads/adconfig"
	"github.com/hanzoai/commerce/models/ads/util"
	"github.com/hanzoai/commerce/models/copy"
	"github.com/hanzoai/commerce/models/fixtures"
	"github.com/hanzoai/commerce/models/media"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/gincontext"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/hanzoai/commerce/util/test/ginclient"

	. "github.com/hanzoai/commerce/marketing/types"
	. "github.com/hanzoai/commerce/util/test/ginkgo"

	marketingApi "github.com/hanzoai/commerce/api/marketing"
)

func Test(t *testing.T) {
	Setup("api/marketing", t)
}

var (
	ctx         ae.Context
	cl          *ginclient.Client
	accessToken string
	db          *datastore.Datastore
	org         *organization.Organization
)

// Setup appengine context
var _ = BeforeSuite(func() {
	adminRequired := middleware.TokenRequired(permission.Admin)

	// Create a new app engine context
	ctx = ae.NewContext()

	// Create mock gin context that we can use with fixtures
	c := gincontext.New(ctx)

	// Run fixtures
	org = fixtures.Organization(c).(*organization.Organization)

	// Save namespaced db
	db = datastore.New(org.Namespaced(ctx))

	// Setup client and add routes for account API tests.
	cl = ginclient.New(ctx)
	marketingApi.Route(cl.Router, adminRequired)

	// Create organization for tests, accessToken
	accessToken, _ := org.GetTokenByName("test-secret-key")
	err := org.Put()
	Expect(err).NotTo(HaveOccurred())

	// Set authorization header for subsequent requests
	cl.Defaults(func(r *http.Request) {
		r.Header.Set("Authorization", accessToken.String)
	})
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("marketing", func() {
	Context("Create", func() {
		It("Should create an adcampaign & associated ad stuff", func() {
			// Demo Engine Test
			cmpgn := *adcampaign.New(db)
			cmpgn.Engine = adcampaign.DemoEngine

			cfg := AdConfigParams{
				*adconfig.New(db),
				[]copy.Copy{
					copy.Copy{
						Type: copy.HeadlineType,
						Text: "1234",
					},
					copy.Copy{
						Type: copy.HeadlineType,
						Text: "ABCD",
					},
				},
				[]copy.Copy{
					copy.Copy{
						Type: copy.ContentType,
						Text: "Numbers are Great",
					},
					copy.Copy{
						Type: copy.ContentType,
						Text: "Letters are Great",
					},
				},
				[]media.Media{
					media.Media{
						Type: media.ImageType,
						URI:  []byte("DataURI1"),
					},
					media.Media{
						Type: media.ImageType,
						URI:  []byte("DataURIA"),
					},
				},
			}
			cfgs := []AdConfigParams{cfg}

			req := CreateInput{
				cmpgn,
				cfgs,
			}

			res := adcampaign.New(db)
			Expect(res.Id_).To(Equal(""))

			log.Debug("Response %s", cl.Post("/marketing", req, &res))
			Expect(res.Id_).ToNot(Equal(""))

			aco, err := util.GetAdConfigs(db, res)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(aco)).To(Equal(1))
			Expect(aco[0].AdCampaignId).To(Equal(res.Id()))

			as, err := util.GetAdSets(db, res)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(as)).To(Equal(1))
			Expect(as[0].AdCampaignId).To(Equal(res.Id()))
			Expect(as[0].AdConfigId).To(Equal(aco[0].Id()))

			a, err := util.GetAds(db, res)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(a)).To(Equal(8))

			for i, ad_ := range a {
				switch i {
				case 0:
					Expect(ad_.Media.Type).To(Equal(media.ImageType))
					Expect(ad_.Media.URI).To(Equal([]byte("DataURI1")))
					Expect(ad_.Media.Usage).To(Equal(media.AdUsage))

					Expect(ad_.Headline.Type).To(Equal(copy.HeadlineType))
					Expect(ad_.Headline.Text).To(Equal("1234"))

					Expect(ad_.Copy.Type).To(Equal(copy.ContentType))
					Expect(ad_.Copy.Text).To(Equal("Numbers are Great"))
				case 1:
					Expect(ad_.Media.Type).To(Equal(media.ImageType))
					Expect(ad_.Media.URI).To(Equal([]byte("DataURI1")))
					Expect(ad_.Media.Usage).To(Equal(media.AdUsage))

					Expect(ad_.Headline.Type).To(Equal(copy.HeadlineType))
					Expect(ad_.Headline.Text).To(Equal("1234"))

					Expect(ad_.Copy.Type).To(Equal(copy.ContentType))
					Expect(ad_.Copy.Text).To(Equal("Letters are Great"))
				case 2:
					Expect(ad_.Media.Type).To(Equal(media.ImageType))
					Expect(ad_.Media.URI).To(Equal([]byte("DataURI1")))
					Expect(ad_.Media.Usage).To(Equal(media.AdUsage))

					Expect(ad_.Headline.Type).To(Equal(copy.HeadlineType))
					Expect(ad_.Headline.Text).To(Equal("ABCD"))

					Expect(ad_.Copy.Type).To(Equal(copy.ContentType))
					Expect(ad_.Copy.Text).To(Equal("Numbers are Great"))
				case 3:
					Expect(ad_.Media.Type).To(Equal(media.ImageType))
					Expect(ad_.Media.URI).To(Equal([]byte("DataURI1")))
					Expect(ad_.Media.Usage).To(Equal(media.AdUsage))

					Expect(ad_.Headline.Type).To(Equal(copy.HeadlineType))
					Expect(ad_.Headline.Text).To(Equal("ABCD"))

					Expect(ad_.Copy.Type).To(Equal(copy.ContentType))
					Expect(ad_.Copy.Text).To(Equal("Letters are Great"))
				case 4:
					Expect(ad_.Media.Type).To(Equal(media.ImageType))
					Expect(ad_.Media.URI).To(Equal([]byte("DataURIA")))
					Expect(ad_.Media.Usage).To(Equal(media.AdUsage))

					Expect(ad_.Headline.Type).To(Equal(copy.HeadlineType))
					Expect(ad_.Headline.Text).To(Equal("1234"))

					Expect(ad_.Copy.Type).To(Equal(copy.ContentType))
					Expect(ad_.Copy.Text).To(Equal("Numbers are Great"))
				case 5:
					Expect(ad_.Media.Type).To(Equal(media.ImageType))
					Expect(ad_.Media.URI).To(Equal([]byte("DataURIA")))
					Expect(ad_.Media.Usage).To(Equal(media.AdUsage))

					Expect(ad_.Headline.Type).To(Equal(copy.HeadlineType))
					Expect(ad_.Headline.Text).To(Equal("1234"))

					Expect(ad_.Copy.Type).To(Equal(copy.ContentType))
					Expect(ad_.Copy.Text).To(Equal("Letters are Great"))
				case 6:
					Expect(ad_.Media.Type).To(Equal(media.ImageType))
					Expect(ad_.Media.URI).To(Equal([]byte("DataURIA")))
					Expect(ad_.Media.Usage).To(Equal(media.AdUsage))

					Expect(ad_.Headline.Type).To(Equal(copy.HeadlineType))
					Expect(ad_.Headline.Text).To(Equal("ABCD"))

					Expect(ad_.Copy.Type).To(Equal(copy.ContentType))
					Expect(ad_.Copy.Text).To(Equal("Numbers are Great"))
				case 7:
					Expect(ad_.Media.Type).To(Equal(media.ImageType))
					Expect(ad_.Media.URI).To(Equal([]byte("DataURIA")))
					Expect(ad_.Media.Usage).To(Equal(media.AdUsage))

					Expect(ad_.Headline.Type).To(Equal(copy.HeadlineType))
					Expect(ad_.Headline.Text).To(Equal("ABCD"))

					Expect(ad_.Copy.Type).To(Equal(copy.ContentType))
					Expect(ad_.Copy.Text).To(Equal("Letters are Great"))
				}
			}
		})
	})
})
