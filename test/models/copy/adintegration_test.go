package test

import (
	"github.com/hanzoai/commerce/models/ads/ad"
	"github.com/hanzoai/commerce/models/ads/adcampaign"
	"github.com/hanzoai/commerce/models/ads/adconfig"
	"github.com/hanzoai/commerce/models/ads/adset"
	"github.com/hanzoai/commerce/models/copy"

	"github.com/hanzoai/commerce/models/ads/util"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

var _ = Describe("models/copy.AdIntegration", func() {
	var a *ad.Ad
	var aca *adcampaign.AdCampaign
	var aco *adconfig.AdConfig
	var as *adset.AdSet
	var m *copy.Copy

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

		m = copy.New(db)
		m.AdCampaignId = aca.Id()
		m.AdConfigId = aco.Id()
		m.AdSetId = as.Id()
		m.AdId = a.Id()
	})

	Context("models/ads/util integration", func() {
		It("Should work", func() {
			aca2, err := util.GetAdCampaign(db, m)
			Expect(err).ToNot(HaveOccurred())

			Expect(aca2.Id()).To(Equal(aca.Id()))

			aco2, err := util.GetAdConfig(db, m)
			Expect(err).ToNot(HaveOccurred())

			Expect(aco2.Id()).To(Equal(aco.Id()))

			as2, err := util.GetAdSet(db, m)
			Expect(err).ToNot(HaveOccurred())

			Expect(as2.Id()).To(Equal(as.Id()))

			a2, err := util.GetAd(db, m)
			Expect(err).ToNot(HaveOccurred())

			Expect(a2.Id()).To(Equal(a.Id()))
		})

		It("Should not work correct", func() {
			c2 := copy.New(db)

			_, err := util.GetAdCampaign(db, c2)
			Expect(err).To(Equal(util.NoAdCampaignFound))

			_, err = util.GetAdConfig(db, c2)
			Expect(err).To(Equal(util.NoAdConfigFound))

			_, err = util.GetAdSet(db, c2)
			Expect(err).To(Equal(util.NoAdSetFound))

			_, err = util.GetAd(db, c2)
			Expect(err).To(Equal(util.NoAdFound))
		})
	})
})
