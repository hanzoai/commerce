package migrations

import (
	"cloud.google.com/go/storage"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/models/organization"

	ds "hanzo.io/datastore"
)

var _ = New("create-buckets",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, org *organization.Organization) {
		ctx := db.Context

		// Sets your Google Cloud Platform project ID.
		productID := "crowdstart-staging"
		if config.Env == "production" {
			projectID := "crowdstart-us"
		} else if config.Env == "sandbox" {
			productID = "crowdstart-sandbox"
		}

		// Creates a client.
		client, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatalf("Failed to create client: %v", err)
		}

		// Sets the name for the new bucket.
		bucketName := "my-new-bucket"

		// Creates a Bucket instance.
		bucket := client.Bucket(bucketName)

		// Creates the new bucket.
		if err := bucket.Create(ctx, projectID, nil); err != nil {
			log.Fatalf("Failed to create bucket: %v", err)
		}

		fmt.Printf("Bucket %v created.\n", bucketName)
	},
)
