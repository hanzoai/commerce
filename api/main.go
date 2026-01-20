package main

import (
	"google.golang.org/appengine"

	a "github.com/hanzoai/commerce/api/api"
	"github.com/hanzoai/commerce/util/router"
)

func main() {
	api := router.New("api")
	a.Route(api)
	appengine.Main()
}
