package fileupload

import (
	"mime/multipart"
	"errors"

	// "golang.org/x/net/context"

	"google.golang.org/appengine"

	// "golang.org/x/oauth2/google"
	// storage "google.golang.org/api/storage/v1"

	"hanzo.io/models/organization"
	// "hanzo.io/util/log"
)

func UploadFile(ctx *context.Context, org *organization.Organization, file multipart.File, header *multipart.FileHeader) (string, error) {
	return "", errors.New("Disabled")
	// client, err := google.DefaultClient(context.Background(), storage.DevstorageFullControlScope)
	// if err != nil {
	// 	log.Error("Unable to get default client: %v", err, ctx)
	//  return "", err
	// }

	// service, err := storage.New(client)
	// if err != nil {
	// 	log.Error("Unable to create storage service: %v", err, ctx)
	// 	return "", err
	// }

	// projectId := appengine.AppID(*ctx)
	// bucketName := org.Name + "-bucket"

	// log.Debug("Project Id %v", projectId, ctx)

	// if _, err := service.Buckets.Get(bucketName).Do(); err != nil {
	// 	if res, err := service.Buckets.Insert(projectId, &storage.Bucket{Name: bucketName}).Do(); err == nil {
	// 		log.Info("Created bucket %v at location %v\n\n", res.Name, res.SelfLink, ctx)
	// 	} else {
	// 		log.Error(service, "Failed creating bucket %s: %v", bucketName, err, ctx)
	// 		return "", err
	// 	}
	// }

	// filename := header.Filename
	// object := &storage.Object{Name: filename}

	// if res, err := service.Objects.Insert(bucketName, object).Media(file).Do(); err != nil {
	// 	log.Error(service, "Objects.Insert failed: %v", err)
	// 	return "", err
	// } else {
	// 	log.Info("Created object %v at location %v\n\n", res.Name, res.SelfLink)
	// 	return res.SelfLink, nil
	// }
}
