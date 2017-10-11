package test

import (
	"testing"

	"hanzo.io/datastore"
	"hanzo.io/marketing"
	"hanzo.io/models/ads/adcampaign"
	"hanzo.io/models/ads/adconfig"
	"hanzo.io/models/ads/util"
	"hanzo.io/models/copy"
	"hanzo.io/models/media"
	"hanzo.io/util/test/ae"

	. "hanzo.io/marketing/types"
	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("marketing", t)
}

var (
	ctx ae.Context
	db  *datastore.Datastore
	ci  CreateInput
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)

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

	ci = CreateInput{
		cmpgn,
		cfgs,
	}
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("marketing", func() {
	Context("Create", func() {
		It("Should properly create a working ad campaign", func() {
			cmpgn, err := marketing.Create(db, ci)
			Expect(err).ToNot(HaveOccurred())
			Expect(cmpgn).ToNot(Equal(nil))
			Expect(cmpgn.Id_).ToNot(Equal(""))

			aco, err := util.GetAdConfigs(db, cmpgn)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(aco)).To(Equal(1))
			Expect(aco[0].AdCampaignId).To(Equal(cmpgn.Id()))

			as, err := util.GetAdSets(db, cmpgn)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(as)).To(Equal(1))
			Expect(as[0].AdCampaignId).To(Equal(cmpgn.Id()))
			Expect(as[0].AdConfigId).To(Equal(aco[0].Id()))

			a, err := util.GetAds(db, cmpgn)
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
