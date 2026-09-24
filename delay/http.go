package delay

import (
	"net/http"
)

// RequestHeaders contains metadata from the task queue request.
// This replaces taskqueue.RequestHeaders from appengine.
type RequestHeaders struct {
	TaskName       string
	TaskRetryCount int64
	QueueName      string
}

// ParseRequestHeaders extracts task queue headers from an HTTP request.
// This is a compatibility function for HTTP-based task invocation.
func ParseRequestHeaders(h http.Header) *RequestHeaders {
	return &RequestHeaders{
		TaskName:       h.Get("X-Task-Name"),
		TaskRetryCount: parseRetryCount(h.Get("X-Task-Retry-Count")),
		QueueName:      h.Get("X-Queue-Name"),
	}
}

func parseRetryCount(s string) int64 {
	if s == "" {
		return 0
	}
	var count int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			count = count*10 + int64(c-'0')
		} else {
			break
		}
	}
	return count
}
