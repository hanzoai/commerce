package test

import (
	"hanzo.io/models/ads/ad"
	"hanzo.io/models/ads/adcampaign"
	"hanzo.io/models/ads/adconfig"
	"hanzo.io/models/ads/adset"
	"hanzo.io/models/media"

	"hanzo.io/models/ads/util"

	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("models/media.AdIntegration", func() {
	var a *ad.Ad
	var aca *adcampaign.AdCampaign
	var aco *adconfig.AdConfig
	var as *adset.AdSet
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

		m = media.New(db)
		m.AdCampaignId = aca.Id()
		m.AdConfigId = aco.Id()
		m.AdSetId = as.Id()
		m.AdId = a.Id()
	})

	Context("models/ads/util integration", func() {
		It("Should work correct", func() {
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
			m2 := media.New(db)

			_, err := util.GetAdCampaign(db, m2)
			Expect(err).To(Equal(util.NoAdCampaignFound))

			_, err = util.GetAdConfig(db, m2)
			Expect(err).To(Equal(util.NoAdConfigFound))

			_, err = util.GetAdSet(db, m2)
			Expect(err).To(Equal(util.NoAdSetFound))

			_, err = util.GetAd(db, m2)
			Expect(err).To(Equal(util.NoAdFound))
		})
	})
})
