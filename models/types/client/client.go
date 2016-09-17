package client

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"appengine"
)

type Client struct {
	Ip        string             `json:"ip,omitempty"`
	UserAgent string             `json:"userAgent,omitempty"`
	Referer   string             `json:"referer,omitempty"`
	Language  string             `json:"language,omitempty"`
	Country   string             `json:"country,omitempty"`
	Region    string             `json:"region,omitempty"`
	City      string             `json:"city,omitempty"`
	GeoPoint  appengine.GeoPoint `json:"geoPoint,omitempty"`
}

func New(c *gin.Context) Client {
	req := c.Request

	// Parse geopoint
	geo := c.Request.Header.Get("X-AppEngine-CitLatLong")
	lat, _ := strconv.ParseFloat(geo, 64)
	long, _ := strconv.ParseFloat(geo, 64)
	geoPoint := appengine.GeoPoint{Lat: lat, Lng: long}

	// Get proxied values from Cloudflare, falling back to AppEngine headers
	ip := req.Header.Get("CF-Connecting-IP")
	if ip == "" {
		ip = req.RemoteAddr
	}

	country := req.Header.Get("CF-IPCountry")
	if country == "" {
		country = req.Header.Get("X-AppEngine-Country")
	}

	return Client{
		Ip:        ip,
		UserAgent: req.UserAgent(),
		Referer:   req.Referer(),
		Language:  req.Header.Get("Accept-Language"),
		Country:   country,
		Region:    req.Header.Get("X-AppEngine-Region"),
		City:      req.Header.Get("X-AppEngine-City"),
		GeoPoint:  geoPoint,
	}
}

func (c Client) Blacklisted() bool {
	// Should check against a blacklist (probably configurable?)
	return false
}
