package netlify

import (
	"time"

	"github.com/netlify/netlify-go"
)

// {"files": {"/index.html": "907d14fb3af2b0d4f18c2d46abe8aedce17367bd"}}
type Digest struct {
	Files map[string]string `json:"files"`
}

// {"deploy_id": "1234", "required": ["907d14fb3af2b0d4f18c2d46abe8aedce17367bd"]}
type Required struct {
	DeployId string   `json:"deploy_id"`
	Required []string `json:"required"`
}

// Represents a Netlify deploy
type Deploy struct {
	Id     string `json:"id"`
	SiteId string `json:"site_id"`
	UserId string `json:"user_id"`

	// State of the deploy (uploading/uploaded/processing/ready/error)
	State string `json:"state"`

	// Cause of error if State is "error"
	ErrorMessage string `json:"error_message"`

	// Shas of files that needs to be uploaded before the deploy is ready
	Required []string `json:"required"`

	DeployUrl     string `json:"deploy_url"`
	SiteUrl       string `json:"url"`
	ScreenshotUrl string `json:"screenshot_url"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newDeploy(d *netlify.Deploy) *Deploy {
	return &Deploy{
		Id:            d.Id,
		SiteId:        d.SiteId,
		UserId:        d.UserId,
		State:         d.State,
		ErrorMessage:  d.ErrorMessage,
		Required:      d.Required,
		DeployUrl:     d.DeployUrl,
		SiteUrl:       d.SiteUrl,
		ScreenshotUrl: d.ScreenshotUrl,
	}
}

// Represents a Netlify site
type Site struct {
	Id     string `json:"id"`
	UserId string `json:"user_id"`

	// These fields can be updated through the API
	Name              string `json:"name"`
	CustomDomain      string `json:"custom_domain"`
	Password          string `json:"password"`
	NotificationEmail string `json:"notification_email"`

	State   string `json:"state"`
	Premium bool   `json:"premium"`
	Claimed bool   `json:"claimed"`

	Url           string `json:"url"`
	AdminUrl      string `json:"admin_url"`
	DeployUrl     string `json:"deploy_url"`
	ScreenshotUrl string `json:"screenshot_url"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newSite(s *netlify.Site) *Site {
	return &Site{
		Id:                s.Id,
		UserId:            s.UserId,
		Name:              s.Name,
		CustomDomain:      s.CustomDomain,
		Password:          s.Password,
		NotificationEmail: s.NotificationEmail,
		State:             s.State,
		Premium:           s.Premium,
		Claimed:           s.Claimed,
		Url:               s.Url,
		AdminUrl:          s.AdminUrl,
		DeployUrl:         s.DeployUrl,
		ScreenshotUrl:     s.ScreenshotUrl,
	}
}
