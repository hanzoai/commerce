package main

import (
	"google.golang.org/appengine"

	"hanzo.io/util/default_"
)

func init() {
	default_.Init()
}

func main() {
	appengine.Main()
}
