package site

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

type Site struct {
	mixin.Model

	Domain string
	Name   string
	Url    string

	Netlify struct {
		AdminUrl           string             `json:"admin_url"`
		Claimed            bool               `json:"claimed"`
		CreatedAt          string             `json:"created_at"`
		CustomDomain       string             `json:"custom_domain"`
		Name               string             `json:"name"`
		NotificationEmail  string             `json:"notification_email"`
		Password           string             `json:"password"`
		Premium            bool               `json:"premium"`
		ProcessingSettings ProcessingSettings `json:"processing_settings"`
		Repo               string             `json:"repo"`
		ScreenshotUrl      string             `json:"screenshot_url"`
		Id                 string             `json:"id"`
		State              string             `json:"state"`
		UpdatedAt          string             `json:"updated_at"`
		Url                string             `json:"url"`
	} `json:"-"`
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
