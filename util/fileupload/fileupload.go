package fileupload

import (
	"errors"
	"mime/multipart"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/models/organization"
)

// UploadFile is a stub — file upload is handled externally (MinIO/S3).
func UploadFile(ctx *gin.Context, org *organization.Organization, file multipart.File, header *multipart.FileHeader) (string, error) {
	return "", errors.New("Disabled")
}
