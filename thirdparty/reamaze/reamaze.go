package reamaze

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/organization"
	"hanzo.io/thirdparty/reamaze/custommodule"
	"hanzo.io/util/log"
	"hanzo.io/util/router"

	"crypto/hmac"
	"crypto/sha256"
)

// CheckMAC reports whether messageMAC is a valid HMAC tag for message.
func checkMAC(message, messageMAC, key []byte) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(messageMAC, expectedMAC)
}

func verifyHMAC(c *context.Context) {
	org := middleware.GetOrganization(c)

	q := c.Request.URL.Query()
	hmacStr := q.Get("hmac")

	q.Del("hmac")
	queryStr := q.Encode()

	if checkMAC([]byte(queryStr), []byte(hmacStr), []byte(org.Reamaze.Secret)) {
		log.Panic("Reamaze signature is not valid", c)
	}
}

func setOrg(c *context.Context) {
	db := datastore.New(c)
	org := organization.New(db)
	brand := c.Request.URL.Query().Get("brand")
	if err := org.GetById(brand); err != nil {
		log.Panic("Organization not specified", c)
	}

	c.Set("organization", org)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("reamaze")

	api.GET("/custommodule", setOrg, verifyHMAC, custommodule.Serve)
}
