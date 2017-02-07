package reamaze

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/organization"
	"crowdstart.com/thirdparty/reamaze/custommodule"
	"crowdstart.com/util/log"
	"crowdstart.com/util/router"

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

func verifyHMAC(c *gin.Context) {
	org := middleware.GetOrganization(c)

	q := c.Request.URL.Query()
	hmacStr := q.Get("hmac")

	q.Del("hmac")
	queryStr := q.Encode()

	if checkMAC([]byte(queryStr), []byte(hmacStr), []byte(org.Reamaze.Secret)) {
		log.Panic("Reamaze signature is not valid", c)
	}
}

func setOrg(c *gin.Context) {
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
