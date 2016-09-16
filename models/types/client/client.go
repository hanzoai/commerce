package client

import "github.com/gin-gonic/gin"

type Client struct {
	Ip        string `json:"ip,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	Language  string `json:"language,omitempty"`
	Referer   string `json:"referer,omitempty"`
	Country   string `json:"country,omitempty"`
	Region    string `json:"region,omitempty"`
	City      string `json:"city,omitempty"`
}

func New(c *gin.Context) Client {
	return Client{
		Ip:        c.Request.RemoteAddr,
		UserAgent: c.Request.UserAgent(),
		Language:  c.Request.Header.Get("Accept-Language"),
		Referer:   c.Request.Referer(),
		Country:   c.Request.Header.Get("X-AppEngine-Country"),
		Region:    c.Request.Header.Get("X-AppEngine-Region"),
		City:      c.Request.Header.Get("X-AppEngine-City"),
	}
}

func (c Client) Blacklisted() bool {
	// Should check against a blacklist (probably configurable?)
	return false
}
