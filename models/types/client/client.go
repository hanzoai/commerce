package client

import (
	"strconv"

	"github.com/zap-proto/zip"
)

type GeoPoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Client struct {
	Ip        string   `json:"ip,omitempty" datastore:",noindex"`
	UserAgent string   `json:"userAgent,omitempty" datastore:",noindex"`
	Referer   string   `json:"referer,omitempty" datastore:",noindex"`
	Language  string   `json:"language,omitempty"`
	Country   string   `json:"country,omitempty"`
	Region    string   `json:"region,omitempty"`
	City      string   `json:"city,omitempty"`
	GeoPoint  GeoPoint `json:"geoPoint,omitempty"`
}

func New(c *zip.Ctx) Client {
	client := Client{
		UserAgent: c.Header("User-Agent"),
		Referer:   c.Header("Referer"),
		Language:  c.Header("Accept-Language"),
	}

	// Check for proxied values from Cloudflare
	client.Ip = c.Header("CF-Connecting-IP")
	client.Country = c.Header("CF-IPCountry")

	// Not behind a proxy
	if client.Ip == "" {
		client.Ip = c.Fiber().IP()
		client.Country = c.Header("X-AppEngine-Country")
		client.Region = c.Header("X-AppEngine-Region")
		client.City = c.Header("X-AppEngine-City")

		// Parse latitude and longitude into geopoint
		geo := c.Header("X-AppEngine-CitLatLong")
		lat, _ := strconv.ParseFloat(geo, 64)
		long, _ := strconv.ParseFloat(geo, 64)
		client.GeoPoint = GeoPoint{Lat: lat, Lng: long}
	}

	return client
}

func (c Client) Blacklisted() bool {
	// Should check against a blacklist (probably configurable?)
	return false
}
