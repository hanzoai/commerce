package preorder

import (
	"crowdstart.io/config"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New(config.Get().PrefixFor("preorder"))

	router.GET("/:token", Get)
}
