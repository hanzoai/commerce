package main

import (
	"google.golang.org/appengine"
	"hanzo.io/util/default_"
)

func main() {
	default_.Init()
	appengine.Main()
}
