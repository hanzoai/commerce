package main

import (
	"google.golang.org/appengine"

	a "hanzo.io/api/api"
	"hanzo.io/util/router"
)

func main() {
	api := router.New("api")
	a.Route(api)
	appengine.Main()
}
