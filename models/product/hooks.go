package product

import (
	"hanzo.io/config"
	"hanzo.io/thirdparty/cloudflare"
	"hanzo.io/util/event"
	"hanzo.io/util/log"
)

func (p *Product) AfterCreate() error {
	return event.Emit(p.Context(), p.Namespace(), "product.created", p)
}

func (p *Product) AfterUpdate(prv *Product) error {
	url := config.UrlFor("api", "/product/", p.Id())
	if err := cloudflare.Purge(p.Context(), url); err != nil {
		log.Error("Failed to purge product %v", err, p.Context())
	} else {
		log.Debug("Successfully purged product", p.Context())
	}
	return event.Emit(p.Context(), p.Namespace(), "product.updated", p)
}

func (p *Product) AfterDelete() error {
	url := config.UrlFor("api", "/product/", p.Id())
	if err := cloudflare.Purge(p.Context(), url); err != nil {
		log.Error("Failed to purge product %v", err, p.Context())
	}
	return event.Emit(p.Context(), p.Namespace(), "product.deleted", p)
}
