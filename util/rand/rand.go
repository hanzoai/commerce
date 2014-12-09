package rand

import (
	"crypto/rand"
	"encoding/base64"
	"strings"

	"crowdstart.io/util/log"
)

// Returns short, url-friendly Id
func ShortId() string {
	size := 8

	rb := make([]byte, size)
	if _, err := rand.Read(rb); err != nil {
		log.Error("Failed to genrate random characters: %v", err)
	}

	return strings.Trim(base64.URLEncoding.EncodeToString(rb), "=")
}
