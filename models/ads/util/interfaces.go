package util

type BelongsToAdCampaign interface {
	GetAdCampaignId() string
}

type BelongsToAdSet interface {
	GetAdSetId() string
}

type BelongsToAdConfig interface {
	GetAdConfigId() string
}

type BelongsToAd interface {
	GetAdId() string
}

type HasAdSets interface {
	GetAdSetSearchFieldAndIds() (string, []string)
}

type HasAdConfigs interface {
	GetAdConfigSearchFieldAndIds() (string, []string)
}

type HasAds interface {
	GetAdSearchFieldAndIds() (string, []string)
}

type HasHeadlines interface {
	GetHeadlineSearchFieldAndIds() (string, []string)
}

type HasCopies interface {
	GetCopySearchFieldAndIds() (string, []string)
}

type HasMedias interface {
	GetMediaSearchFieldAndIds() (string, []string)
}
