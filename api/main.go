package main

import (
	"google.golang.org/appengine"
	"hanzo.io/util/router"
)

func main() {
	api := router.New("api")
	Route(api)
	appengine.Main()
}
