package main

import (
	"google.golang.org/appengine"
	"github.com/hanzoai/commerce/util/default_"
)

func main() {
	default_.Init()
	appengine.Main()
}
