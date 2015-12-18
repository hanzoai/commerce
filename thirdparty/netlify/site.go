package netlify

import (
	"time"

	"github.com/netlify/netlify-go"
)

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
