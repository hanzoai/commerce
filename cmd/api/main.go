package main

import (
	"log"
	"os"

	"github.com/zap-proto/zip"

	a "github.com/hanzoai/commerce/api"
	"github.com/hanzoai/commerce/util/router"
)

func main() {
	app := zip.New(zip.Config{AppName: "commerce-api", DisableStartupMessage: true})
	a.Route(router.New(app, "api"))

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(app.Listen("http://:" + port))
}
