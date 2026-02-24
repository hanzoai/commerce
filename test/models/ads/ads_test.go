package test

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/ads/ad"
	"github.com/hanzoai/commerce/models/ads/adcampaign"
	"github.com/hanzoai/commerce/models/ads/adconfig"
	"github.com/hanzoai/commerce/models/ads/adset"
	"github.com/hanzoai/commerce/models/ads/util"
	"github.com/hanzoai/commerce/models/copy"
	"github.com/hanzoai/commerce/models/media"
	"github.com/hanzoai/commerce/util/test/ae"

	. "github.com/hanzoai/commerce/models/ads"
	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/ads", t)
}

var (
	ctx ae.Context
	db  *datastore.Datastore
)

// Setup test context
var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)
})

// Tear-down test context
var _ = AfterSuite(func() {
	ctx.Close()
})

var _ = Describe("models/ads/adcampaign", func() {
	var a *ad.Ad
	var aca *adcampaign.AdCampaign
	var aco *adconfig.AdConfig
	var as *adset.AdSet
	var h *copy.Copy
	var c *copy.Copy
	var m *media.Media

	Before(func() {
		aca = adcampaign.New(db)
		aca.MustCreate()

		aco = adconfig.New(db)
		aco.AdCampaignId = aca.Id()
		aco.MustCreate()

		as = adset.New(db)
		as.AdCampaignId = aca.Id()
		as.AdConfigId = aco.Id()
		as.MustCreate()

		a = ad.New(db)
		a.AdCampaignId = aca.Id()
		a.AdConfigId = aco.Id()
		a.AdSetId = as.Id()
		a.MustCreate()

		h = copy.New(db)
		h.AdCampaignId = aca.Id()
		h.AdConfigId = aco.Id()
		h.AdSetId = as.Id()
		h.AdId = a.Id()
		h.Type = copy.HeadlineType
		h.MustCreate()

		c = copy.New(db)
		c.AdCampaignId = aca.Id()
		c.AdConfigId = aco.Id()
		c.AdSetId = as.Id()
		c.AdId = a.Id()
		c.Type = copy.ContentType
		c.MustCreate()

		m = media.New(db)
		m.AdCampaignId = aca.Id()
		m.AdConfigId = aco.Id()
		m.AdSetId = as.Id()
		m.AdId = a.Id()
		m.MustCreate()

		a.Headline = *h
		a.Copy = *c
		a.Media = *m
		a.MustUpdate()
	})

	Context("ad/adcampaign/adset.New", func() {
		It("Should use defaults", func() {
			Expect(aca.Status).To(Equal(PendingStatus))
			Expect(as.Status).To(Equal(PendingStatus))
			Expect(a.Status).To(Equal(PendingStatus))
		})
	})

	Context("util.*", func() {
		It("Should work correctly", func() {
			aca1, err := util.GetAdCampaign(db, aco)
			Expect(err).ToNot(HaveOccurred())
			aca2, err := util.GetAdCampaign(db, as)
			Expect(err).ToNot(HaveOccurred())
			aca3, err := util.GetAdCampaign(db, a)
			Expect(err).ToNot(HaveOccurred())

			Expect(aca1.Id()).To(Equal(aca.Id()))
			Expect(aca2.Id()).To(Equal(aca.Id()))
			Expect(aca3.Id()).To(Equal(aca.Id()))

			aco1, err := util.GetAdConfigs(db, aca)
			Expect(err).ToNot(HaveOccurred())
			aco2, err := util.GetAdConfig(db, as)
			Expect(err).ToNot(HaveOccurred())
			aco3, err := util.GetAdConfig(db, a)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(aco1)).To(Equal(1))
			Expect(aco1[0].Id()).To(Equal(aco.Id()))
			Expect(aco2.Id()).To(Equal(aco.Id()))
			Expect(aco3.Id()).To(Equal(aco.Id()))

			as1, err := util.GetAdSets(db, aca)
			Expect(err).ToNot(HaveOccurred())
			as2, err := util.GetAdSets(db, aco)
			Expect(err).ToNot(HaveOccurred())
			as3, err := util.GetAdSet(db, a)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(as1)).To(Equal(1))
			Expect(as1[0].Id()).To(Equal(as.Id()))
			Expect(len(as2)).To(Equal(1))
			Expect(as2[0].Id()).To(Equal(as.Id()))
			Expect(as3.Id()).To(Equal(as.Id()))

			a1, err := util.GetAds(db, aca)
			Expect(err).ToNot(HaveOccurred())
			a2, err := util.GetAds(db, aco)
			Expect(err).ToNot(HaveOccurred())
			a3, err := util.GetAds(db, as)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(a1)).To(Equal(1))
			Expect(a1[0].Id()).To(Equal(a.Id()))
			Expect(len(a2)).To(Equal(1))
			Expect(a2[0].Id()).To(Equal(a.Id()))
			Expect(len(a3)).To(Equal(1))
			Expect(a3[0].Id()).To(Equal(a.Id()))

			h1, err := util.GetHeadlines(db, aca)
			Expect(err).ToNot(HaveOccurred())
			h2, err := util.GetHeadlines(db, aco)
			Expect(err).ToNot(HaveOccurred())
			h3, err := util.GetHeadlines(db, as)
			Expect(err).ToNot(HaveOccurred())
			h4, err := util.GetHeadlines(db, a)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(h1)).To(Equal(1))
			Expect(h1[0].Id()).To(Equal(h.Id()))
			Expect(len(h2)).To(Equal(1))
			Expect(h2[0].Id()).To(Equal(h.Id()))
			Expect(len(h3)).To(Equal(1))
			Expect(h3[0].Id()).To(Equal(h.Id()))
			Expect(len(h4)).To(Equal(1))
			Expect(h4[0].Id()).To(Equal(h.Id()))

			c1, err := util.GetCopies(db, aca)
			Expect(err).ToNot(HaveOccurred())
			c2, err := util.GetCopies(db, aco)
			Expect(err).ToNot(HaveOccurred())
			c3, err := util.GetCopies(db, as)
			Expect(err).ToNot(HaveOccurred())
			c4, err := util.GetCopies(db, a)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(c1)).To(Equal(1))
			Expect(c1[0].Id()).To(Equal(c.Id()))
			Expect(len(c2)).To(Equal(1))
			Expect(c2[0].Id()).To(Equal(c.Id()))
			Expect(len(c3)).To(Equal(1))
			Expect(c3[0].Id()).To(Equal(c.Id()))
			Expect(len(c4)).To(Equal(1))
			Expect(c4[0].Id()).To(Equal(c.Id()))

			m1, err := util.GetMedias(db, aca)
			Expect(err).ToNot(HaveOccurred())
			m2, err := util.GetMedias(db, aco)
			Expect(err).ToNot(HaveOccurred())
			m3, err := util.GetMedias(db, as)
			Expect(err).ToNot(HaveOccurred())
			m4, err := util.GetMedias(db, a)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(m1)).To(Equal(1))
			Expect(m1[0].Id()).To(Equal(m.Id()))
			Expect(len(m2)).To(Equal(1))
			Expect(m2[0].Id()).To(Equal(m.Id()))
			Expect(len(m3)).To(Equal(1))
			Expect(m3[0].Id()).To(Equal(m.Id()))
			Expect(len(m4)).To(Equal(1))
			Expect(m4[0].Id()).To(Equal(m.Id()))
		})

		It("Should not work correctly", func() {
			aco.AdCampaignId = ""
			as.AdCampaignId = ""
			a.AdCampaignId = ""

			as.AdConfigId = ""
			a.AdConfigId = ""

			a.AdSetId = ""
			aco.MustUpdate()
			as.MustUpdate()
			a.MustUpdate()

			_, err := util.GetAdCampaign(db, aco)
			Expect(err).To(Equal(util.NoAdCampaignFound))
			_, err = util.GetAdCampaign(db, as)
			Expect(err).To(Equal(util.NoAdCampaignFound))
			_, err = util.GetAdCampaign(db, a)
			Expect(err).To(Equal(util.NoAdCampaignFound))

			aco1, err := util.GetAdConfigs(db, aca)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(aco1)).To(Equal(0))
			_, err = util.GetAdConfig(db, as)
			Expect(err).To(Equal(util.NoAdConfigFound))
			_, err = util.GetAdConfig(db, a)
			Expect(err).To(Equal(util.NoAdConfigFound))

			as1, err := util.GetAdSets(db, aca)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(as1)).To(Equal(0))
			as2, err := util.GetAdSets(db, aco)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(as2)).To(Equal(0))
			_, err = util.GetAdSet(db, a)
			Expect(err).To(Equal(util.NoAdSetFound))

			a1, err := util.GetAds(db, aca)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(a1)).To(Equal(0))
			a2, err := util.GetAds(db, aco)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(a2)).To(Equal(0))
			a3, err := util.GetAds(db, as)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(a3)).To(Equal(0))
		})
	})
})
