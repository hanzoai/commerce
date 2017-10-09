package util

type HasCampaign interface {
	GetCampaignId() string
}

type HasAdset interface {
	GetAdsetId() string
}

type HasAdConfig interface {
	GetAdConfigId() string
}

type HasAd interface {
	GetAdId() string
}

type HasHeadlines interface {
	GetHeadlineSearchFieldAndId() (string, string)
}

type HasCopies interface {
	GetCopySearchFieldAndId() (string, string)
}

type HasMedias interface {
	GetMediaSearchFieldAndId() (string, string)
}
