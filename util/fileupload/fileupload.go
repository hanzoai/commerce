package fileupload

// import (
// 	"golang.org/x/net/context"

// 	"appengine"

// 	"golang.org/x/oauth2/google"
// 	storage "google.golang.org/api/storage/v1"

// 	"crowdstart.com/models/organization"
// 	"crowdstart.com/util/log"
// )

// func UploadFile(c *appengine.Context, org *organization.Organization) string {
// 	client, err := google.DefaultClient(context.Background(), scope)
// 	if err != nil {
// 		log.Error("Unable to get default client: %v", err, c)
// 	}

// 	service, err := storage.New(client)

// 	return ""
// }
