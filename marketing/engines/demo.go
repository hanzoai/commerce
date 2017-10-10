package engines

import (
	"hanzo.io/datastore"
	"hanzo.io/marketing/types"
	"hanzo.io/models/ads/ad"
	"hanzo.io/models/ads/adcampaign"
	"hanzo.io/models/ads/adconfig"
	"hanzo.io/models/ads/adset"
	// "hanzo.io/models/copy"
	"hanzo.io/models/media"
)

type DemoEngine struct{}

func (d DemoEngine) Create(db *datastore.Datastore, ci types.CreateInput) (types.CreateOutput, error) {
	co := types.CreateOutput{}

	cmpgn := adcampaign.New(db)
	co.AdCampaign = cmpgn
	co.Entities = []interface{}{cmpgn}
	// co.AdCampaign = adcampaign.New(db)
	// co.AdConfigs = make([]*adconfig.AdConfig, len(ci.AdConfigs))
	// co.AdSets = make([]*adset.AdSet, len(ci.AdConfigs))
	// co.Ads = make([]*ad.Ad, len(ci.AdConfigs))
	// co.Headlines = make([]*copy.Copy, 0)
	// co.Copies = make([]*copy.Copy, 0)
	// co.Medias = make([]*media.Media, 0)

	for _, adcfgparams := range ci.AdConfigs {
		cfg := adconfig.New(db)
		cfg.AdCampaignId = cmpgn.Id()
		cfg.FacebookAdTypePlacements = adcfgparams.FacebookAdTypePlacements

		co.Entities = append(co.Entities, cfg)

		as := adset.New(db)
		as.AdCampaignId = cmpgn.Id()
		as.AdConfigId = cfg.Id()

		co.Entities = append(co.Entities, as)

		for _, headline := range adcfgparams.Headlines {
			co.Entities = append(co.Entities, &headline)
			headline.Init(db)
			headline.AdCampaignId = cmpgn.Id()
			headline.AdConfigId = cfg.Id()
			headline.AdSetId = as.Id()
		}

		for _, cop := range adcfgparams.Copies {
			co.Entities = append(co.Entities, &cop)
			cop.Init(db)
			cop.AdCampaignId = cmpgn.Id()
			cop.AdConfigId = cfg.Id()
			cop.AdSetId = as.Id()
		}

		for _, med := range adcfgparams.Medias {
			co.Entities = append(co.Entities, &med)
			med.Init(db)
			med.Usage = media.AdUsage
			med.AdCampaignId = cmpgn.Id()
			med.AdConfigId = cfg.Id()
			med.AdSetId = as.Id()

			m := med.Fork()
			co.Entities = append(co.Entities, m)

			for _, headline := range adcfgparams.Headlines {
				headline.Init(db)
				h := headline.Fork()
				co.Entities = append(co.Entities, h)

				for _, cop := range adcfgparams.Copies {
					cop.Init(db)
					c := cop.Fork()
					co.Entities = append(co.Entities, c)

					a := ad.New(db)
					h.AdId = a.Id()
					c.AdId = a.Id()
					m.AdId = m.Id()

					a.AdCampaignId = cmpgn.Id()
					a.AdSetId = as.Id()
					a.AdConfigId = cfg.Id()
					a.Headline = *h
					a.Copy = *c
					a.Media = *m

					co.Entities = append(co.Entities, a)
				}
			}
		}
	}

	return co, nil
}

func (d DemoEngine) StartAdCampaign(*adcampaign.AdCampaign) error {
	return nil
}

func (d DemoEngine) StopAdCampaign(*adcampaign.AdCampaign) error {
	return nil
}

func (d DemoEngine) StartAdSet(*adset.AdSet) error {
	return nil
}

func (d DemoEngine) StopAdSet(*adset.AdSet) error {
	return nil
}

func (d DemoEngine) StartAd(*ad.Ad) error {
	return nil
}

func (d DemoEngine) StopAd(*ad.Ad) error {
	return nil
}

func (d DemoEngine) Next(*adcampaign.AdCampaign, *adconfig.AdConfig, *adset.AdSet, *ad.Ad) error {
	return nil
}
