package main

import (
	"google.golang.org/appengine"

	"github.com/hanzoai/commerce/util/default_"
)

func init() {
	default_.Init()
}

func main() {
	appengine.Main()
}
