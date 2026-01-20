// Package analytics provides HTTP endpoints for event collection.
// This supports astley.js automatic tracking and manual event submission.
package analytics

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/util/router"
)

// Handler handles analytics event collection.
type Handler struct {
	emitter *events.Emitter
}

// NewHandler creates a new analytics handler.
func NewHandler(emitter *events.Emitter) *Handler {
	return &Handler{emitter: emitter}
}

// Route sets up analytics routes.
func (h *Handler) Route(r router.Router) {
	g := r.Group("analytics")
	{
		// Event collection endpoints
		g.POST("/event", h.handleEvent)
		g.POST("/events", h.handleBatch)
		g.POST("/pageview", h.handlePageView)
		g.POST("/identify", h.handleIdentify)

		// astley.js specific endpoints
		g.POST("/ast", h.handleAST)
		g.POST("/element", h.handleElement)
		g.POST("/section", h.handleSection)

		// Pixel tracking (for email/ads)
		g.GET("/pixel.gif", h.handlePixel)

		// AI/Cloud event endpoints
		g.POST("/ai/message", h.handleAIMessage)
		g.POST("/ai/completion", h.handleAICompletion)
	}
}

// EventRequest is the standard event request format.
type EventRequest struct {
	// Core
	Event      string `json:"event" binding:"required"`
	DistinctID string `json:"distinct_id"`
	Timestamp  string `json:"timestamp"`

	// Organization context
	OrganizationID string `json:"organization_id"`
	ProjectID      string `json:"project_id"`

	// Session
	SessionID string `json:"session_id"`
	VisitID   string `json:"visit_id"`

	// Properties
	Properties map[string]interface{} `json:"properties"`

	// Web context
	URL      string `json:"url"`
	Referrer string `json:"referrer"`

	// AST context (astley.js)
	Context string `json:"@context"`
	Type    string `json:"@type"`

	// Element tracking
	ElementID       string `json:"element_id"`
	ElementType     string `json:"element_type"`
	ElementSelector string `json:"element_selector"`
	ElementText     string `json:"element_text"`
	ElementHref     string `json:"element_href"`

	// Section tracking
	SectionName string `json:"section_name"`
	SectionType string `json:"section_type"`
	SectionID   string `json:"section_id"`

	// Page info
	PageTitle       string `json:"page_title"`
	PageDescription string `json:"page_description"`
	PageType        string `json:"page_type"`

	// Component hierarchy
	ComponentPath string `json:"component_path"`
	ComponentData string `json:"component_data"`

	// AI/Cloud
	ModelProvider string  `json:"model_provider"`
	ModelName     string  `json:"model_name"`
	TokenCount    int     `json:"token_count"`
	TokenPrice    float64 `json:"token_price"`
	PromptTokens  int     `json:"prompt_tokens"`
	OutputTokens  int     `json:"output_tokens"`

	// Commerce
	OrderID   string  `json:"order_id"`
	ProductID string  `json:"product_id"`
	CartID    string  `json:"cart_id"`
	Revenue   float64 `json:"revenue"`
	Quantity  int     `json:"quantity"`
}

// BatchRequest is a batch of events.
type BatchRequest struct {
	Events []EventRequest `json:"events" binding:"required"`
}

// ASTRequest is the astley.js page AST request format.
type ASTRequest struct {
	Context string `json:"@context"` // hanzo.ai/schema
	Type    string `json:"@type"`    // Website, WebsiteSection, etc.

	// Page metadata
	Head struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		OG          struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Image       string `json:"image"`
		} `json:"og"`
	} `json:"head"`

	// Sections
	Sections []struct {
		Name    string `json:"name"`
		Type    string `json:"type"` // hero, block, cta
		ID      string `json:"id"`
		Class   string `json:"class"`
		Content []struct {
			Type string `json:"type"` // text, image, link, button
			Text string `json:"text"`
			Href string `json:"href"`
			Src  string `json:"src"`
			Alt  string `json:"alt"`
		} `json:"content"`
	} `json:"sections"`

	// User context
	DistinctID     string `json:"distinct_id"`
	OrganizationID string `json:"organization_id"`
	SessionID      string `json:"session_id"`
	URL            string `json:"url"`
}

// handleEvent processes a single event.
func (h *Handler) handleEvent(c *gin.Context) {
	var req EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	event := h.buildRawEvent(c, &req)
	if err := h.emitter.EmitRaw(c.Request.Context(), event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to emit event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleBatch processes a batch of events.
func (h *Handler) handleBatch(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, eventReq := range req.Events {
		event := h.buildRawEvent(c, &eventReq)
		h.emitter.EmitRaw(c.Request.Context(), event)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "count": len(req.Events)})
}

// handlePageView processes a page view event.
func (h *Handler) handlePageView(c *gin.Context) {
	var req EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Event = "$pageview"
	event := h.buildRawEvent(c, &req)
	if err := h.emitter.EmitRaw(c.Request.Context(), event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to emit event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleIdentify processes an identify event.
func (h *Handler) handleIdentify(c *gin.Context) {
	var req struct {
		DistinctID       string                 `json:"distinct_id" binding:"required"`
		OrganizationID   string                 `json:"organization_id"`
		PersonProperties map[string]interface{} `json:"person_properties"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	event := &events.RawEvent{
		Event:            "$identify",
		DistinctID:       req.DistinctID,
		OrganizationID:   req.OrganizationID,
		PersonProperties: req.PersonProperties,
		IP:               c.ClientIP(),
		UserAgent:        c.Request.UserAgent(),
		Timestamp:        time.Now(),
		SentAt:           time.Now(),
		Lib:              "hanzo-analytics",
	}

	if err := h.emitter.EmitRaw(c.Request.Context(), event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to emit event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleAST processes astley.js page AST data.
func (h *Handler) handleAST(c *gin.Context) {
	var req ASTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Emit page view with AST context
	pageEvent := &events.RawEvent{
		Event:           "$pageview",
		DistinctID:      req.DistinctID,
		OrganizationID:  req.OrganizationID,
		SessionID:       req.SessionID,
		URL:             req.URL,
		ASTContext:      req.Context,
		ASTType:         req.Type,
		PageTitle:       req.Head.Title,
		PageDescription: req.Head.Description,
		IP:              c.ClientIP(),
		UserAgent:       c.Request.UserAgent(),
		Timestamp:       time.Now(),
		SentAt:          time.Now(),
		Lib:             "astley.js",
	}

	// Extract URL path
	if parsedURL, err := url.Parse(req.URL); err == nil {
		pageEvent.URLPath = parsedURL.Path
		pageEvent.Hostname = parsedURL.Host
	}

	h.emitter.EmitRaw(c.Request.Context(), pageEvent)

	// Emit section events for tracking
	for _, section := range req.Sections {
		sectionEvent := &events.RawEvent{
			Event:          "section_viewed",
			DistinctID:     req.DistinctID,
			OrganizationID: req.OrganizationID,
			SessionID:      req.SessionID,
			URL:            req.URL,
			ASTContext:     req.Context,
			SectionName:    section.Name,
			SectionType:    section.Type,
			SectionID:      section.ID,
			IP:             c.ClientIP(),
			UserAgent:      c.Request.UserAgent(),
			Timestamp:      time.Now(),
			SentAt:         time.Now(),
			Lib:            "astley.js",
		}

		// Serialize section content as component data
		if contentJSON, err := json.Marshal(section.Content); err == nil {
			sectionEvent.ComponentData = string(contentJSON)
		}

		h.emitter.EmitRaw(c.Request.Context(), sectionEvent)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"sections": len(req.Sections),
	})
}

// handleElement processes element interaction events.
func (h *Handler) handleElement(c *gin.Context) {
	var req EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default event name based on element type
	if req.Event == "" {
		switch req.ElementType {
		case "button":
			req.Event = "button_clicked"
		case "link":
			req.Event = "link_clicked"
		case "form":
			req.Event = "form_submitted"
		case "input":
			req.Event = "input_changed"
		default:
			req.Event = "element_interaction"
		}
	}

	event := h.buildRawEvent(c, &req)
	event.Lib = "astley.js"

	if err := h.emitter.EmitRaw(c.Request.Context(), event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to emit event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleSection processes section visibility events.
func (h *Handler) handleSection(c *gin.Context) {
	var req EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Event == "" {
		req.Event = "section_viewed"
	}

	event := h.buildRawEvent(c, &req)
	event.Lib = "astley.js"

	if err := h.emitter.EmitRaw(c.Request.Context(), event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to emit event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handlePixel handles pixel tracking requests (for email opens, ads, etc).
func (h *Handler) handlePixel(c *gin.Context) {
	event := &events.RawEvent{
		Event:          "pixel_view",
		DistinctID:     c.Query("uid"),
		OrganizationID: c.Query("oid"),
		SessionID:      c.Query("sid"),
		Properties: map[string]interface{}{
			"source":      c.Query("src"),
			"campaign_id": c.Query("cid"),
			"email_id":    c.Query("eid"),
		},
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Referrer:  c.Request.Referer(),
		Timestamp: time.Now(),
		SentAt:    time.Now(),
		Lib:       "hanzo-pixel",
	}

	h.emitter.EmitRaw(c.Request.Context(), event)

	// Return 1x1 transparent GIF
	c.Header("Content-Type", "image/gif")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Data(http.StatusOK, "image/gif", transparentGIF)
}

// handleAIMessage handles AI message events from Cloud.
func (h *Handler) handleAIMessage(c *gin.Context) {
	var req struct {
		DistinctID     string                 `json:"distinct_id" binding:"required"`
		OrganizationID string                 `json:"organization_id"`
		ChatID         string                 `json:"chat_id"`
		MessageID      string                 `json:"message_id"`
		Role           string                 `json:"role"` // user, assistant, system
		ModelProvider  string                 `json:"model_provider"`
		ModelName      string                 `json:"model_name"`
		TokenCount     int                    `json:"token_count"`
		PromptTokens   int                    `json:"prompt_tokens"`
		OutputTokens   int                    `json:"output_tokens"`
		TokenPrice     float64                `json:"token_price"`
		Properties     map[string]interface{} `json:"properties"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	event := &events.RawEvent{
		Event:          "ai.message.created",
		DistinctID:     req.DistinctID,
		OrganizationID: req.OrganizationID,
		SessionID:      req.ChatID,
		Properties:     req.Properties,
		ModelProvider:  req.ModelProvider,
		ModelName:      req.ModelName,
		TokenCount:     req.TokenCount,
		PromptTokens:   req.PromptTokens,
		OutputTokens:   req.OutputTokens,
		TokenPrice:     req.TokenPrice,
		IP:             c.ClientIP(),
		UserAgent:      c.Request.UserAgent(),
		Timestamp:      time.Now(),
		SentAt:         time.Now(),
		Lib:            "hanzo-cloud",
	}

	// Add role and message ID to properties
	if event.Properties == nil {
		event.Properties = make(map[string]interface{})
	}
	event.Properties["role"] = req.Role
	event.Properties["message_id"] = req.MessageID

	if err := h.emitter.EmitRaw(c.Request.Context(), event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to emit event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleAICompletion handles AI completion events (aggregated).
func (h *Handler) handleAICompletion(c *gin.Context) {
	var req struct {
		DistinctID     string  `json:"distinct_id" binding:"required"`
		OrganizationID string  `json:"organization_id"`
		ChatID         string  `json:"chat_id"`
		ModelProvider  string  `json:"model_provider"`
		ModelName      string  `json:"model_name"`
		PromptTokens   int     `json:"prompt_tokens"`
		OutputTokens   int     `json:"output_tokens"`
		TotalTokens    int     `json:"total_tokens"`
		Price          float64 `json:"price"`
		DurationMs     int64   `json:"duration_ms"`
		Success        bool    `json:"success"`
		ErrorMessage   string  `json:"error_message,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	event := &events.RawEvent{
		Event:          "ai.completion",
		DistinctID:     req.DistinctID,
		OrganizationID: req.OrganizationID,
		SessionID:      req.ChatID,
		ModelProvider:  req.ModelProvider,
		ModelName:      req.ModelName,
		PromptTokens:   req.PromptTokens,
		OutputTokens:   req.OutputTokens,
		TokenCount:     req.TotalTokens,
		TokenPrice:     req.Price,
		Properties: map[string]interface{}{
			"duration_ms":   req.DurationMs,
			"success":       req.Success,
			"error_message": req.ErrorMessage,
		},
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Timestamp: time.Now(),
		SentAt:    time.Now(),
		Lib:       "hanzo-cloud",
	}

	if err := h.emitter.EmitRaw(c.Request.Context(), event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to emit event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// buildRawEvent creates a RawEvent from the request.
func (h *Handler) buildRawEvent(c *gin.Context, req *EventRequest) *events.RawEvent {
	event := &events.RawEvent{
		Event:           req.Event,
		DistinctID:      req.DistinctID,
		OrganizationID:  req.OrganizationID,
		ProjectID:       req.ProjectID,
		SessionID:       req.SessionID,
		VisitID:         req.VisitID,
		Properties:      req.Properties,
		URL:             req.URL,
		Referrer:        req.Referrer,
		ASTContext:      req.Context,
		ASTType:         req.Type,
		PageTitle:       req.PageTitle,
		PageDescription: req.PageDescription,
		PageType:        req.PageType,
		ElementID:       req.ElementID,
		ElementType:     req.ElementType,
		ElementSelector: req.ElementSelector,
		ElementText:     req.ElementText,
		ElementHref:     req.ElementHref,
		SectionName:     req.SectionName,
		SectionType:     req.SectionType,
		SectionID:       req.SectionID,
		ComponentPath:   req.ComponentPath,
		ComponentData:   req.ComponentData,
		ModelProvider:   req.ModelProvider,
		ModelName:       req.ModelName,
		TokenCount:      req.TokenCount,
		TokenPrice:      req.TokenPrice,
		PromptTokens:    req.PromptTokens,
		OutputTokens:    req.OutputTokens,
		OrderID:         req.OrderID,
		ProductID:       req.ProductID,
		CartID:          req.CartID,
		Revenue:         req.Revenue,
		Quantity:        req.Quantity,
		IP:              c.ClientIP(),
		UserAgent:       c.Request.UserAgent(),
		Timestamp:       time.Now(),
		SentAt:          time.Now(),
		Lib:             "hanzo-analytics",
	}

	// Parse timestamp if provided
	if req.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, req.Timestamp); err == nil {
			event.Timestamp = t
		}
	}

	// Extract URL components
	if req.URL != "" {
		if parsedURL, err := url.Parse(req.URL); err == nil {
			event.URLPath = parsedURL.Path
			event.Hostname = parsedURL.Host

			// Extract UTM parameters
			query := parsedURL.Query()
			event.UTMSource = query.Get("utm_source")
			event.UTMMedium = query.Get("utm_medium")
			event.UTMCampaign = query.Get("utm_campaign")
			event.UTMContent = query.Get("utm_content")
			event.UTMTerm = query.Get("utm_term")

			// Extract click IDs
			event.GCLID = query.Get("gclid")
			event.FBCLID = query.Get("fbclid")
			event.MSCLID = query.Get("msclid")
		}
	}

	// Extract referrer domain
	if req.Referrer != "" {
		if parsedRef, err := url.Parse(req.Referrer); err == nil {
			event.ReferrerDomain = parsedRef.Host
		}
	}

	// Set distinct ID from IP if not provided
	if event.DistinctID == "" {
		event.DistinctID = c.ClientIP()
	}

	// Parse user agent for device info
	ua := c.Request.UserAgent()
	event.Browser, event.BrowserVersion = parseUserAgentBrowser(ua)
	event.OS, event.OSVersion = parseUserAgentOS(ua)
	event.DeviceType = parseDeviceType(ua)

	return event
}

// parseUserAgentBrowser extracts browser name and version from user agent.
func parseUserAgentBrowser(ua string) (string, string) {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "chrome"):
		return "Chrome", ""
	case strings.Contains(ua, "firefox"):
		return "Firefox", ""
	case strings.Contains(ua, "safari"):
		return "Safari", ""
	case strings.Contains(ua, "edge"):
		return "Edge", ""
	case strings.Contains(ua, "opera"):
		return "Opera", ""
	default:
		return "Other", ""
	}
}

// parseUserAgentOS extracts OS name and version from user agent.
func parseUserAgentOS(ua string) (string, string) {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "windows"):
		return "Windows", ""
	case strings.Contains(ua, "mac os"):
		return "macOS", ""
	case strings.Contains(ua, "linux"):
		return "Linux", ""
	case strings.Contains(ua, "android"):
		return "Android", ""
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		return "iOS", ""
	default:
		return "Other", ""
	}
}

// parseDeviceType determines device type from user agent.
func parseDeviceType(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "mobile"):
		return "mobile"
	case strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad"):
		return "tablet"
	default:
		return "desktop"
	}
}

// 1x1 transparent GIF
var transparentGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00,
	0x80, 0x00, 0x00, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x2c,
	0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02,
	0x02, 0x44, 0x01, 0x00, 0x3b,
}
