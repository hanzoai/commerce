package fileupload

import (
	"errors"
	"mime/multipart"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/organization"
)

// UploadFile is a stub — file upload is handled externally (MinIO/S3).
func UploadFile(ctx *zip.Ctx, org *organization.Organization, file multipart.File, header *multipart.FileHeader) (string, error) {
	return "", errors.New("Disabled")
}
