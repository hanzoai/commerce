package site

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

type Site struct {
	mixin.Model
	SiteId             string             `json:"id"`
	Premium            bool               `json:"premium"`
	Claimed            bool               `json:"claimed"`
	NotificationEmail  string             `json:"notification_email"`
	Url                string             `json:"url"`
	AdminUrl           string             `json:"admin_url"`
	ScreenshotUrl      string             `json:"screenshot_url"`
	CreatedAt          string             `json:"created_at"`
	State              string             `json:"state"`
	UpdatedAt          string             `json:"updated_at"`
	Name               string             `json:"name"`
	CustomDomain       string             `json:"custom_domain"`
	Password           string             `json:"password"`
	Repo               string             `json:"repo"`
	ProcessingSettings ProcessingSettings `json:"processing_settings"`
}

type ProcessingSettings struct {
	Css    Css    `json: "css"`
	Js     Js     `json: "js"`
	Html   Html   `json: "html"`
	Images Images `json: "images"`
}

type Css struct {
	Bundle bool `json: "bundle"`
	Minify bool `json: "minify"`
}

type Js struct {
	Bundle bool `json: "bundle"`
	Minify bool `json: "minify"`
}

type Html struct {
	PrettyUrls    bool `json: "pretty_urls"`
	CanonicalUrls bool `json: "canonical_urls"`
}

type Images struct {
	Optimize bool `json: "optimize"`
}

func (s *Site) Init() {
}

func New(db *datastore.Datastore) *Site {
	s := new(Site)
	s.Init()
	s.Model = mixin.Model{Db: db, Entity: s}
	return s
}

func (s Site) Kind() string {
	return "site"
}

func (s Site) Document() mixin.Document {
	return &Document{}
}
