package util

import (
	"errors"

	"hanzo.io/datastore"
	"hanzo.io/models/ads/ad"
	"hanzo.io/models/ads/adcampaign"
	"hanzo.io/models/ads/adconfig"
	"hanzo.io/models/ads/adset"
	"hanzo.io/models/copy"
	"hanzo.io/models/media"
)

var NoAdCampaignFound = errors.New("No AdCampaign Found")
var NoAdConfigFound = errors.New("No AdConfig Found")
var NoAdSetFound = errors.New("No AdSet Found")
var NoAdFound = errors.New("No Ad Found")

func GetAdCampaign(db *datastore.Datastore, h BelongsToAdCampaign) (*adcampaign.AdCampaign, error) {
	if id := h.GetAdCampaignId(); id == "" {
		return nil, NoAdCampaignFound
	} else {
		a := adcampaign.New(db)
		err := a.GetById(id)
		return a, err
	}
}

func GetAdSet(db *datastore.Datastore, h BelongsToAdSet) (*adset.AdSet, error) {
	if id := h.GetAdSetId(); id == "" {
		return nil, NoAdSetFound
	} else {
		a := adset.New(db)
		err := a.GetById(id)
		return a, err
	}
}

func GetAdConfig(db *datastore.Datastore, h BelongsToAdConfig) (*adconfig.AdConfig, error) {
	if id := h.GetAdConfigId(); id == "" {
		return nil, NoAdConfigFound
	} else {
		a := adconfig.New(db)
		err := a.GetById(id)
		return a, err
	}
}

func GetAd(db *datastore.Datastore, h BelongsToAd) (*ad.Ad, error) {
	if id := h.GetAdId(); id == "" {
		return nil, NoAdFound
	} else {
		a := ad.New(db)
		err := a.GetById(id)
		return a, err
	}
}

func GetAdSets(db *datastore.Datastore, h HasAdSets) ([]*adset.AdSet, error) {
	field, keys := h.GetAdSetSearchFieldAndIds()
	results := make([]*adset.AdSet, 0)
	part := make([]*adset.AdSet, 0)
	for _, key := range keys {
		if _, err := adset.Query(db).Filter(field+"=", key).GetAll(&part); err != nil {
			return nil, err
		}
		results = append(results, part...)
	}
	return results, nil
}

func GetAdConfigs(db *datastore.Datastore, h HasAdConfigs) ([]*adconfig.AdConfig, error) {
	field, keys := h.GetAdConfigSearchFieldAndIds()
	results := make([]*adconfig.AdConfig, 0)
	part := make([]*adconfig.AdConfig, 0)
	for _, key := range keys {
		if _, err := adconfig.Query(db).Filter(field+"=", key).GetAll(&part); err != nil {
			return nil, err
		}
		results = append(results, part...)
	}
	return results, nil
}

func GetAds(db *datastore.Datastore, h HasAds) ([]*ad.Ad, error) {
	field, keys := h.GetAdSearchFieldAndIds()
	results := make([]*ad.Ad, 0)
	part := make([]*ad.Ad, 0)
	for _, key := range keys {
		if _, err := ad.Query(db).Filter(field+"=", key).GetAll(&part); err != nil {
			return nil, err
		}
		results = append(results, part...)
	}
	return results, nil
}

func GetCopies(db *datastore.Datastore, h HasCopies) ([]*copy.Copy, error) {
	field, keys := h.GetCopySearchFieldAndIds()
	results := make([]*copy.Copy, 0)
	part := make([]*copy.Copy, 0)
	for _, key := range keys {
		if _, err := copy.Query(db).Filter("Type=", copy.ContentType).Filter(field+"=", key).GetAll(&part); err != nil {
			return nil, err
		}
		results = append(results, part...)
	}
	return results, nil
}

func GetHeadlines(db *datastore.Datastore, h HasHeadlines) ([]*copy.Copy, error) {
	field, keys := h.GetHeadlineSearchFieldAndIds()
	results := make([]*copy.Copy, 0)
	part := make([]*copy.Copy, 0)
	for _, key := range keys {
		if _, err := copy.Query(db).Filter("Type=", copy.HeadlineType).Filter(field+"=", key).GetAll(&part); err != nil {
			return nil, err
		}
		results = append(results, part...)
	}
	return results, nil
}

func GetMedias(db *datastore.Datastore, h HasMedias) ([]*media.Media, error) {
	field, keys := h.GetMediaSearchFieldAndIds()
	results := make([]*media.Media, 0)
	part := make([]*media.Media, 0)
	for _, key := range keys {
		if _, err := media.Query(db).Filter(field+"=", key).GetAll(&part); err != nil {
			return nil, err
		}
		results = append(results, part...)
	}
	return results, nil
}
