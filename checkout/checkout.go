package checkout

import (
	"crowdstart.io/config"
	"crowdstart.io/util/router"
)

func init() {
	// Initialising stripe client
	Sc.Init(config.Get().Stripe.APISecret, nil)

	router := router.New("checkout")

	router.POST("/", checkout)
	router.POST("/authorize", authorize)
	router.GET("/complete", complete)
}
