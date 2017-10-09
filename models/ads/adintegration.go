package ads

// AdIntegration
type AdIntegration struct {
	AdId         string `json:"AdId,omitempty"`
	AdConfigId   string `json:"AdConfigId,omitempty"`
	AdSetId      string `json:"AdSetId,omitempty"`
	AdCampaignId string `json:"AdCampaignId,omitempty"`
}

func (a *AdIntegration) GetAdId() string {
	return a.AdId
}

func (a *AdIntegration) GetAdConfigId() string {
	return a.AdConfigId
}

func (a *AdIntegration) GetAdSetId() string {
	return a.AdSetId
}

func (a *AdIntegration) GetAdCampaignId() string {
	return a.AdCampaignId
}
