// Package events provides ClickHouse schema for unified analytics.
package events

// Schema contains ClickHouse table definitions for unified analytics.
// Both Insights (PostHog) and Analytics (Umami) read from these tables.
const Schema = `
-- Unified events table compatible with both PostHog and Umami query patterns
CREATE TABLE IF NOT EXISTS commerce.events (
    -- Core identifiers
    event_id UUID DEFAULT generateUUIDv4(),
    distinct_id String,
    event String,

    -- Timestamps
    timestamp DateTime64(3) DEFAULT now64(3),
    sent_at DateTime64(3) DEFAULT now64(3),
    created_at DateTime64(3) DEFAULT now64(3),

    -- Organization/tenant
    organization_id String,
    project_id String DEFAULT '',

    -- Session tracking (Umami-style)
    session_id String DEFAULT '',
    visit_id String DEFAULT '',

    -- Properties (JSON for flexibility)
    properties String DEFAULT '{}',

    -- User properties (for $identify events)
    person_properties String DEFAULT '{}',

    -- Group properties (for $groupidentify events)
    group_type String DEFAULT '',
    group_key String DEFAULT '',
    group_properties String DEFAULT '{}',

    -- Web analytics fields (Umami-compatible)
    url String DEFAULT '',
    url_path String DEFAULT '',
    referrer String DEFAULT '',
    referrer_domain String DEFAULT '',
    hostname String DEFAULT '',

    -- Device/browser info
    browser String DEFAULT '',
    browser_version String DEFAULT '',
    os String DEFAULT '',
    os_version String DEFAULT '',
    device String DEFAULT '',
    device_type LowCardinality(String) DEFAULT '',
    screen String DEFAULT '',
    language String DEFAULT '',

    -- Geo (MaxMind-style)
    country LowCardinality(String) DEFAULT '',
    region String DEFAULT '',
    city String DEFAULT '',

    -- UTM tracking
    utm_source String DEFAULT '',
    utm_medium String DEFAULT '',
    utm_campaign String DEFAULT '',
    utm_content String DEFAULT '',
    utm_term String DEFAULT '',

    -- Click IDs
    gclid String DEFAULT '',
    fbclid String DEFAULT '',
    msclkid String DEFAULT '',

    -- Request metadata
    ip String DEFAULT '',
    user_agent String DEFAULT '',

    -- Commerce-specific
    order_id String DEFAULT '',
    product_id String DEFAULT '',
    cart_id String DEFAULT '',
    revenue Decimal64(4) DEFAULT 0,
    quantity UInt32 DEFAULT 0,

    -- AST/Structured Data (astley.js support)
    ast_context String DEFAULT '',
    ast_type String DEFAULT '',
    page_title String DEFAULT '',
    page_description String DEFAULT '',
    page_type LowCardinality(String) DEFAULT '',

    -- Element interaction tracking
    element_id String DEFAULT '',
    element_type LowCardinality(String) DEFAULT '',
    element_selector String DEFAULT '',
    element_text String DEFAULT '',
    element_href String DEFAULT '',

    -- Section tracking (astley.js WebsiteSection)
    section_name String DEFAULT '',
    section_type LowCardinality(String) DEFAULT '',
    section_id String DEFAULT '',

    -- Component hierarchy
    component_path String DEFAULT '',
    component_data String DEFAULT '',

    -- AI/Cloud events
    model_provider LowCardinality(String) DEFAULT '',
    model_name String DEFAULT '',
    token_count UInt32 DEFAULT 0,
    token_price Decimal64(6) DEFAULT 0,
    prompt_tokens UInt32 DEFAULT 0,
    output_tokens UInt32 DEFAULT 0,

    -- Library info
    lib String DEFAULT 'hanzo-commerce',
    lib_version String DEFAULT '',

    -- Partitioning key
    _partition_date Date DEFAULT toDate(timestamp)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(_partition_date)
ORDER BY (organization_id, toStartOfHour(timestamp), distinct_id, session_id, event_id)
SETTINGS index_granularity = 8192;

-- Materialized view for hourly stats (Umami query optimization)
CREATE MATERIALIZED VIEW IF NOT EXISTS commerce.events_hourly_mv
TO commerce.events_hourly
AS SELECT
    organization_id,
    toStartOfHour(timestamp) as hour,
    event,
    url_path,
    referrer_domain,
    country,
    device_type,
    browser,
    os,
    count() as event_count,
    uniqExact(distinct_id) as unique_users,
    uniqExact(session_id) as unique_sessions,
    sum(revenue) as total_revenue
FROM commerce.events
GROUP BY organization_id, hour, event, url_path, referrer_domain, country, device_type, browser, os;

-- Aggregated hourly stats table
CREATE TABLE IF NOT EXISTS commerce.events_hourly (
    organization_id String,
    hour DateTime,
    event String,
    url_path String,
    referrer_domain String,
    country LowCardinality(String),
    device_type LowCardinality(String),
    browser String,
    os String,
    event_count UInt64,
    unique_users UInt64,
    unique_sessions UInt64,
    total_revenue Decimal64(4)
)
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (organization_id, hour, event, url_path, referrer_domain, country, device_type, browser, os);

-- Person profiles (PostHog-style)
CREATE TABLE IF NOT EXISTS commerce.persons (
    distinct_id String,
    organization_id String,
    properties String DEFAULT '{}',
    created_at DateTime64(3) DEFAULT now64(3),
    updated_at DateTime64(3) DEFAULT now64(3),

    -- Denormalized common fields
    email String DEFAULT '',
    name String DEFAULT '',

    _partition_date Date DEFAULT toDate(created_at)
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(_partition_date)
ORDER BY (organization_id, distinct_id);

-- Sessions table (Umami-style)
CREATE TABLE IF NOT EXISTS commerce.sessions (
    session_id String,
    distinct_id String,
    organization_id String,

    -- Session timing
    started_at DateTime64(3),
    ended_at DateTime64(3),
    duration_seconds UInt32 DEFAULT 0,

    -- Entry/exit
    entry_url String DEFAULT '',
    exit_url String DEFAULT '',

    -- Aggregates
    pageview_count UInt32 DEFAULT 0,
    event_count UInt32 DEFAULT 0,
    is_bounce UInt8 DEFAULT 0,

    -- Device info (denormalized)
    browser String DEFAULT '',
    os String DEFAULT '',
    device_type LowCardinality(String) DEFAULT '',
    country LowCardinality(String) DEFAULT '',

    _partition_date Date DEFAULT toDate(started_at)
)
ENGINE = ReplacingMergeTree(ended_at)
PARTITION BY toYYYYMM(_partition_date)
ORDER BY (organization_id, session_id);

-- Groups/Organizations (PostHog-style)
CREATE TABLE IF NOT EXISTS commerce.groups (
    group_type String,
    group_key String,
    organization_id String,
    properties String DEFAULT '{}',
    created_at DateTime64(3) DEFAULT now64(3),
    updated_at DateTime64(3) DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (organization_id, group_type, group_key);
`

// StandardEvents defines event names used across the platform.
var StandardEvents = struct {
	// Page/Screen events
	PageView   string
	ScreenView string

	// Identification
	Identify      string
	GroupIdentify string
	Alias         string

	// Commerce events
	ProductViewed   string
	ProductAdded    string
	ProductRemoved  string
	CartViewed      string
	CheckoutStarted string
	CheckoutStep    string
	OrderCompleted  string
	OrderRefunded   string

	// User lifecycle
	SignedUp  string
	SignedIn  string
	SignedOut string

	// Engagement
	FeatureUsed  string
	ButtonClick  string
	FormSubmit   string
	SearchQuery  string

	// AST/UI events (astley.js)
	SectionViewed      string
	ElementInteraction string
	LinkClicked        string
	InputChanged       string
	ScrollDepth        string
	VisibilityChange   string

	// AI/Cloud events
	AIMessageCreated string
	AIChatStarted    string
	AICompletion     string
	AITokensConsumed string
	AIModelInvoked   string
	AIError          string

	// Pixel tracking
	PixelView string

	// API tracking
	APIRequest string
	Exception  string
}{
	PageView:        "$pageview",
	ScreenView:      "$screen",
	Identify:        "$identify",
	GroupIdentify:   "$groupidentify",
	Alias:           "$create_alias",
	ProductViewed:   "product_viewed",
	ProductAdded:    "product_added",
	ProductRemoved:  "product_removed",
	CartViewed:      "cart_viewed",
	CheckoutStarted: "checkout_started",
	CheckoutStep:    "checkout_step",
	OrderCompleted:  "order_completed",
	OrderRefunded:   "order_refunded",
	SignedUp:        "signed_up",
	SignedIn:        "signed_in",
	SignedOut:       "signed_out",
	FeatureUsed:     "feature_used",
	ButtonClick:     "button_clicked",
	FormSubmit:      "form_submitted",
	SearchQuery:     "search_query",
	// AST/UI events (astley.js)
	SectionViewed:      "section_viewed",
	ElementInteraction: "element_interaction",
	LinkClicked:        "link_clicked",
	InputChanged:       "input_changed",
	ScrollDepth:        "scroll_depth",
	VisibilityChange:   "visibility_change",
	// AI/Cloud events
	AIMessageCreated: "ai.message.created",
	AIChatStarted:    "ai.chat.started",
	AICompletion:     "ai.completion",
	AITokensConsumed: "ai.tokens.consumed",
	AIModelInvoked:   "ai.model.invoked",
	AIError:          "ai.error",
	// Pixel tracking
	PixelView: "pixel_view",
	// API tracking
	APIRequest: "$api_request",
	Exception:  "$exception",
}
