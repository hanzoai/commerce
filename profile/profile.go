package profile

import (
	"crowdstart.io/util/router"
	"net/http"
)

func init() {
	router := router.New()

	router.POST("/login", login)
	http.Handle("/checkout/", router)
}
