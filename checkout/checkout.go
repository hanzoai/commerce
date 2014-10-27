package checkout

import (
	"crowdstart.io/util/router"
	"net/http"
)

func init() {
	router := router.New()

	router.GET("/checkout",  showCheckout)
	router.GET("/checkout/", showCheckout)
	router.POST("/checkout", processCheckout)

	http.Handle("/checkout", router)
}

