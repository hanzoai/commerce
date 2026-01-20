// Package insights provides Gin middleware for automatic event tracking.
package insights

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// Middleware returns a Gin middleware that tracks HTTP requests.
func Middleware(client *Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Track the request
		duration := time.Since(start)

		// Get user ID from context if available
		distinctID := c.GetString("user_id")
		if distinctID == "" {
			distinctID = c.GetString("session_id")
		}
		if distinctID == "" {
			distinctID = c.ClientIP()
		}

		// Build properties
		props := map[string]interface{}{
			"$current_url":     c.Request.URL.String(),
			"method":           c.Request.Method,
			"path":             c.Request.URL.Path,
			"status_code":      c.Writer.Status(),
			"duration_ms":      duration.Milliseconds(),
			"$ip":              c.ClientIP(),
			"$user_agent":      c.Request.UserAgent(),
			"$referrer":        c.Request.Referer(),
			"$lib":             "hanzo-commerce",
			"$lib_method":      "server",
			"content_length":   c.Writer.Size(),
			"request_id":       c.GetString("request_id"),
			"organization_id":  c.GetString("organization_id"),
		}

		// Track API request
		client.Capture(&Event{
			DistinctID: distinctID,
			Event:      "$api_request",
			Properties: props,
		})
	}
}

// ErrorMiddleware tracks errors that occur during request processing.
func ErrorMiddleware(client *Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check for errors
		if len(c.Errors) > 0 {
			distinctID := c.GetString("user_id")
			if distinctID == "" {
				distinctID = c.ClientIP()
			}

			for _, err := range c.Errors {
				client.Capture(&Event{
					DistinctID: distinctID,
					Event:      "$exception",
					Properties: map[string]interface{}{
						"$exception_message": err.Error(),
						"$exception_type":    fmt.Sprintf("%v", err.Type),
						"path":               c.Request.URL.Path,
						"method":             c.Request.Method,
						"$lib":               "hanzo-commerce",
					},
				})
			}
		}
	}
}

// TrackEvent is a helper to track custom events from handlers.
func TrackEvent(c *gin.Context, client *Client, eventName string, properties map[string]interface{}) {
	distinctID := c.GetString("user_id")
	if distinctID == "" {
		distinctID = c.GetString("session_id")
	}
	if distinctID == "" {
		distinctID = c.ClientIP()
	}

	// Merge with default properties
	props := map[string]interface{}{
		"$ip":              c.ClientIP(),
		"$user_agent":      c.Request.UserAgent(),
		"$lib":             "hanzo-commerce",
		"organization_id":  c.GetString("organization_id"),
	}
	for k, v := range properties {
		props[k] = v
	}

	client.Capture(&Event{
		DistinctID: distinctID,
		Event:      eventName,
		Properties: props,
	})
}
