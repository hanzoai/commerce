package main

import (
	"log"
	"os"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/util/default_"
)

func main() {
	app := zip.New(zip.Config{AppName: "commerce-default", DisableStartupMessage: true})
	default_.Init(app)

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on :%s", port)
	log.Fatal(app.Listen("http://:" + port))
}
