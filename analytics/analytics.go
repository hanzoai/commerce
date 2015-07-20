package api

import "crowdstart.com/util/router"

func init() {
	analytics := router.New("analytics")
	analytics.GET("/", router.Ok)
	analytics.HEAD("/", router.Empty)
}
