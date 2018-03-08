package coupon

import (
	"hanzo.io/config"
	"hanzo.io/thirdparty/cloudflare"
	"hanzo.io/util/event"
	"hanzo.io/util/log"
)

// Hooks
func (c *Coupon) AfterCreate() error {
	return event.Emit(c.Context(), c.Namespace(), "coupon.created", c)
}

func (c *Coupon) AfterUpdate(previous *Coupon) error {
	url := config.UrlFor("api", "/coupon/", c.Id())
	if err := cloudflare.Purge(c.Context(), url); err != nil {
		log.Error("Failed to purge coupon %v", err, c.Context())
	}
	return event.Emit(c.Context(), c.Namespace(), "coupon.updated", c)
}

func (c *Coupon) AfterDelete() error {
	url := config.UrlFor("api", "/coupon/", c.Id())
	if err := cloudflare.Purge(c.Context(), url); err != nil {
		log.Error("Failed to purge coupon %v", err, c.Context())
	}
	return event.Emit(c.Context(), c.Namespace(), "coupon.deleted", c)
}
