package reamaze

import (
	"crypto/hmac"
	"crypto/sha256"
	"net/url"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/thirdparty/reamaze/custommodule"
)

// CheckMAC reports whether messageMAC is a valid HMAC tag for message.
func checkMAC(message, messageMAC, key []byte) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(messageMAC, expectedMAC)
}

func verifyHMAC(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	q, _ := url.ParseQuery(string(c.Fiber().Request().URI().QueryString()))
	hmacStr := q.Get("hmac")

	q.Del("hmac")
	queryStr := q.Encode()

	if checkMAC([]byte(queryStr), []byte(hmacStr), []byte(org.Reamaze.Secret)) {
		log.Panic("Reamaze signature is not valid", c)
	}

	return c.Next()
}

func setOrg(c *zip.Ctx) error {
	db := datastore.New(c.Context())
	org := organization.New(db)
	brand := c.Query("brand")
	if err := org.GetById(brand); err != nil {
		log.Panic("Organization not specified", c)
	}

	c.Locals("organization", org)
	return c.Next()
}

func Route(router zip.Router, args ...zip.Handler) {
	api := router.Group("reamaze")

	api.Get("/custommodule", setOrg, verifyHMAC, custommodule.Serve)
}
