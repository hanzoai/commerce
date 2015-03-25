package rand

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
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

func Int32() int32 {
	var n int32
	binary.Read(rand.Reader, binary.LittleEndian, &n)
	return n
}

func Int() int {
	return int(Int32())
}

func Int64() int64 {
	return int64(Int32())
}
