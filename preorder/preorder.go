package preorder

import (
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("/")

	router.GET("/preorder/:slug", Get)
}
