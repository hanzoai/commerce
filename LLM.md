# Commerce - LLM Context Document

## Overview

Hanzo Commerce is a multi-tenant e-commerce platform that has been fully modernized from a Google App Engine monolith to a standalone binary with embedded SQLite. All App Engine dependencies have been removed.

## Architecture

### Current State (Modernization Complete)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Commerce App v1.33.0                          │
├─────────────────────────────────────────────────────────────────────────┤
│  CLI (Cobra)    │  HTTP (Gin)    │  Hooks System  │  Events Emitter    │
├─────────────────────────────────────────────────────────────────────────┤
│  User SQLite    │  Org SQLite    │  Datastore     │  Insights+Analytics│
│  + sqlite-vec   │  + sqlite-vec  │  (ClickHouse)  │  (PostHog+Umami)   │
│  Per-user data  │  Shared tenant │  Deep analytics│  Product+Web       │
└─────────────────────────────────────────────────────────────────────────┘
```

### Database Backends

The `db/` package supports multiple backends:

| Backend | Use Case | Vector Search |
|---------|----------|---------------|
| **SQLite** | Per-user/org isolation, edge deployment | sqlite-vec |
| **PostgreSQL** | Shared deployments, scaling | pgvector |
| **MongoDB/FerretDB** | Document-oriented, flexible schema | Atlas Search |
| **Hanzo Datastore** | Deep analytics, parallel queries (ClickHouse) | - |

### Database Layers

1. **User SQLite** (`data/users/{userID}/data.db`)
   - Per-user data isolation
   - sqlite-vec for vector embeddings
   - Fast local queries
   - WAL mode for concurrency

2. **Organization SQLite** (`data/orgs/{orgID}/data.db`)
   - Shared tenant data
   - Organization-level settings
   - Multi-user access within org

3. **PostgreSQL** (Alternative to SQLite)
   - Shared multi-tenant deployment
   - pgvector for vector search
   - Schema-based tenant isolation
   - JSONB for flexible data

4. **MongoDB/FerretDB** (Alternative)
   - Document-oriented storage
   - FerretDB uses PostgreSQL/SQLite backend
   - MongoDB-compatible API

5. **Hanzo Datastore** (ClickHouse, Optional)
   - Deep analytics queries
   - Parallel processing
   - Event streaming
   - Connected via `hanzo/datastore-go`

## Key Directories

```
commerce/
├── cmd/commerce/              # CLI entry point
├── commerce.go                # Main app framework
├── db/                        # Database abstraction
│   ├── db.go                  # Interfaces and Manager
│   ├── sqlite.go              # SQLite implementation
│   ├── postgres.go            # PostgreSQL implementation
│   ├── mongo.go               # MongoDB/FerretDB implementation
│   ├── query.go               # Query builder
│   └── datastore.go           # Hanzo Datastore (ClickHouse) connector
├── hooks/                     # Hook system (Base-compatible)
│   ├── hooks.go               # Core hook registry
│   ├── event.go               # Event types and Resolver interface
│   └── tagged.go              # TaggedHook for filtered execution
├── insights/                  # PostHog integration
│   ├── insights.go            # Insights client
│   └── middleware.go          # Gin middleware for tracking
├── events/                    # Unified event forwarding
│   └── events.go              # Multi-backend event emitter
├── integrations/              # Third-party integrations
│   └── analyticsapi/          # Hanzo Analytics client
├── infra/                     # Infrastructure clients
├── api/                       # HTTP API handlers
├── models/                    # Data models
├── datastore/                 # Datastore wrapper
├── config/                    # Configuration
└── middleware/                # HTTP middleware
```

## Running Commerce

### As Standalone Binary

```bash
# Development
go run cmd/commerce/main.go serve --dev

# Production
./commerce serve 0.0.0.0:80

# With full analytics stack
COMMERCE_DATASTORE="native://localhost:9000/commerce" \
INSIGHTS_ENABLED=true \
INSIGHTS_ENDPOINT="https://insights.hanzo.ai" \
INSIGHTS_API_KEY="phc_..." \
ANALYTICS_ENABLED=true \
ANALYTICS_ENDPOINT="https://analytics.hanzo.ai" \
ANALYTICS_WEBSITE_ID="website-uuid" \
./commerce serve
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `COMMERCE_DIR` | Data directory | `./commerce_data` |
| `COMMERCE_DEV` | Development mode | `false` |
| `COMMERCE_SECRET` | Encryption secret | `change-me-in-production` |
| `COMMERCE_HTTP` | HTTP address | `127.0.0.1:8090` |
| `COMMERCE_DATASTORE` | Hanzo Datastore DSN | (disabled) |
| `INSIGHTS_ENABLED` | Enable Insights (PostHog) | `false` |
| `INSIGHTS_ENDPOINT` | Insights API URL | `https://insights.hanzo.ai` |
| `INSIGHTS_API_KEY` | Insights project API key | - |
| `ANALYTICS_ENABLED` | Enable Analytics (Umami-like) | `false` |
| `ANALYTICS_ENDPOINT` | Analytics API URL | `https://analytics.hanzo.ai` |
| `ANALYTICS_WEBSITE_ID` | Analytics website ID | - |

## Hook System

Commerce uses a hook system compatible with Hanzo Base patterns:

### Core Concepts

- **Hook[T]**: Generic, thread-safe collection of handlers
- **TaggedHook[T]**: Filters execution based on event tags
- **Resolver**: Interface for events that can chain to next handler
- **Handler**: Has ID, Priority, and Func

### Event Types

```go
// Base event with chaining support
type Event struct {
    nextFunc func() error
}

func (e *Event) Next() error { ... }

// Tagged event for filtered hooks
type TaggedEvent struct {
    Event
    tags []string
}
```

### Registering Hooks

```go
// On order creation (using TaggedHook pattern from Base)
app.Hooks.OnModelCreate("Order").Bind(&hooks.Handler[*hooks.ModelEvent]{
    ID:       "validateInventory",
    Priority: 10,
    Func: func(e *hooks.ModelEvent) error {
        order := e.Model.(*Order)
        // Validate inventory before create
        return e.Next() // MUST call to continue chain
    },
})

// On server start
app.Hooks.OnServe().Bind(&hooks.Handler[*hooks.AppEvent]{
    ID: "startCron",
    Func: func(e *hooks.AppEvent) error {
        // Start background jobs
        return e.Next()
    },
})
```

## Events System

Commerce includes a unified events system with shared datastore storage:

### Architecture

```
Commerce App
    │
    ▼
┌─────────────────┐
│  Event Emitter  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   ClickHouse    │ ◄── Single source of truth
│   (Datastore)   │
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
Insights  Analytics
(PostHog) (Umami)
```

### Unified Datastore (Primary)

Events are written directly to ClickHouse via the Hanzo Datastore. Both Insights and Analytics query from the same tables, ensuring data consistency:

- **commerce.events** - Main event table (MergeTree)
- **commerce.events_hourly** - Aggregated stats (SummingMergeTree)
- **commerce.persons** - User profiles (ReplacingMergeTree)
- **commerce.sessions** - Session data (ReplacingMergeTree)
- **commerce.groups** - Group analytics (ReplacingMergeTree)

### HTTP Forwarding (Optional)

For hybrid deployments, events can also be forwarded via HTTP:

1. **Hanzo Insights (PostHog fork)** - Product analytics
   - User behavior tracking
   - Funnels and conversions
   - Feature flags
   - Session recording

2. **Hanzo Analytics (Umami-like)** - Web analytics
   - Page views and sessions
   - Privacy-focused tracking
   - UTM parameter tracking
   - Referrer analysis

### Using the Events Emitter

```go
// Events are automatically forwarded to configured backends
emitter := app.Events

// Track order completed
emitter.EmitOrderCompleted(ctx, &events.Order{
    ID:     "ord_123",
    UserID: "usr_456",
    Total:  99.99,
    Items:  items,
})

// Track product viewed
emitter.EmitProductViewed(ctx, userID, &events.Product{
    ID:    "prod_789",
    Name:  "Widget",
    Price: 29.99,
})

// Track page view
emitter.EmitPageView(ctx, &events.PageView{
    URL:    "https://shop.example.com/products",
    Title:  "Products",
    UserID: userID,
})
```

### Insights Middleware

```go
// Add to Gin router for automatic HTTP tracking
app.Router.Use(insights.Middleware(app.Events.insightsClient))
```

### Analytics API Endpoints

Commerce exposes a unified analytics API for event collection:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/analytics/event` | POST | Single event |
| `/api/v1/analytics/events` | POST | Batch events |
| `/api/v1/analytics/pageview` | POST | Page view |
| `/api/v1/analytics/identify` | POST | User identification |
| `/api/v1/analytics/ast` | POST | astley.js page AST |
| `/api/v1/analytics/element` | POST | Element interaction |
| `/api/v1/analytics/section` | POST | Section visibility |
| `/api/v1/analytics/pixel.gif` | GET | Pixel tracking |
| `/api/v1/analytics/ai/message` | POST | AI message event |
| `/api/v1/analytics/ai/completion` | POST | AI completion event |

### astley.js Integration

astley.js can send structured page data using JSON-LD format:

```javascript
// Send page AST to Commerce
fetch('/api/v1/analytics/ast', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    '@context': 'hanzo.ai/schema',
    '@type': 'Website',
    head: {
      title: 'Product Page',
      description: 'View our products'
    },
    sections: [
      { name: 'hero', type: 'hero', id: 'hero-section' },
      { name: 'products', type: 'block', id: 'product-grid' }
    ],
    distinct_id: 'user_123',
    organization_id: 'org_456',
    url: window.location.href
  })
});

// Track element interaction
fetch('/api/v1/analytics/element', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    element_type: 'button',
    element_id: 'add-to-cart',
    element_text: 'Add to Cart',
    section_name: 'product-detail',
    distinct_id: 'user_123'
  })
});
```

### Cloud AI Integration

Hanzo Cloud can send AI events to the same datastore:

```go
// From Cloud: track AI message
http.Post("https://commerce.example.com/api/v1/analytics/ai/message",
    "application/json",
    strings.NewReader(`{
        "distinct_id": "user_123",
        "organization_id": "org_456",
        "chat_id": "chat_789",
        "model_provider": "openai",
        "model_name": "gpt-4",
        "token_count": 1250,
        "token_price": 0.015
    }`))
```

## Database Usage

### Getting User Database

```go
// Get database for a specific user
db, err := app.DB.User("user123")
if err != nil {
    return err
}

// Query user's orders
var orders []Order
_, err = db.Query("Order").
    Filter("Status=", "pending").
    Order("-CreatedAt").
    Limit(10).
    GetAll(ctx, &orders)
```

### Vector Search

```go
// Store embedding
err := db.PutVector(ctx, "Product", "prod123", embedding, map[string]interface{}{
    "name": "Cool Product",
    "category": "electronics",
})

// Search similar items
results, err := db.VectorSearch(ctx, &db.VectorSearchOptions{
    Kind:     "Product",
    Vector:   queryEmbedding,
    Limit:    10,
    MinScore: 0.7,
})
```

### Hanzo Datastore Queries

```go
// Get Hanzo Datastore for deep analytics
datastore := app.DB.Datastore()

// Run analytics query
var stats []SalesStats
err := datastore.Select(ctx, &stats, `
    SELECT
        toDate(created_at) as date,
        count() as orders,
        sum(total) as revenue
    FROM orders
    WHERE created_at > now() - interval 30 day
    GROUP BY date
    ORDER BY date
`)
```

## Migration Status

All migration phases are complete:

1. ✅ **Phase 1**: Create new `db/` package with SQLite backend
2. ✅ **Phase 2**: Create standalone CLI and app framework
3. ✅ **Phase 3**: Migrate models to new `db.DB` interface
4. ✅ **Phase 4**: Remove App Engine dependencies (145 files, 0 imports remain)
5. ✅ **Phase 5**: Integrate `hanzo/iam` for auth
6. ✅ **Phase 6**: Integrate datastore-go for ClickHouse
7. ✅ **Phase 7**: Add AI recommendations via Cloud-Backend
8. ✅ **Phase 8**: Create deployment configuration
9. ✅ **Phase 9**: Events system (Insights + Analytics integration)

**Commerce is now a fully standalone binary with zero App Engine dependencies.**

## Dependencies

### Core Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/gin-gonic/gin` - HTTP framework
- `github.com/hanzoai/datastore-go` - ClickHouse connector

### Infrastructure Dependencies

- `github.com/redis/go-redis/v9` - Valkey/Redis client
- `github.com/qdrant/go-client` - Qdrant vector DB
- `github.com/minio/minio-go/v7` - MinIO object storage
- `github.com/meilisearch/meilisearch-go` - Meilisearch client
- `github.com/nats-io/nats.go` - NATS pub/sub
- `go.temporal.io/sdk` - Temporal workflow engine

## Testing

```bash
# Run all tests
go test ./...

# Run specific package
go test ./db/...

# With verbose output
go test -v ./...
```

## Hanzo Ecosystem Integration

Commerce integrates with the broader Hanzo ecosystem:

### Services Matrix

| Service | Path | Tech | Status | Integration |
|---------|------|------|--------|-------------|
| **IAM (hanzo.id)** | `~/work/hanzo/iam` | Go/React | ✅ Production | OAuth2/OIDC |
| **Cloud** | `~/work/hanzo/cloud` | Go/Beego | ✅ Production | AI/LLM APIs |
| **Cloud-Backend** | `~/work/hanzo/cloud-backend` | Rust/Tokio | ✅ Production | Inference |
| **Datastore** | `~/work/hanzo/datastore-go` | Go | ✅ Production | ClickHouse |
| **Base** | `~/work/hanzo/base` | Go | ✅ Production | Reference |
| **Insights** | `~/work/hanzo/insights` | PostHog fork | ✅ Ready | Product analytics |
| **Analytics** | `~/work/hanzo/analytics` | Next.js | ✅ Ready | Web analytics |

### Growth + Automation Stack

Commerce is designed to work with the full Hanzo Growth stack:

```
┌─────────────────────────────────────────────────────────────┐
│                     Hanzo Data Layer                        │
├─────────────────────────────────────────────────────────────┤
│  BigQuery      │  ClickHouse     │  PostgreSQL              │
│  (Analytics)   │  (Real-time)    │  (Operational)           │
└─────────────────────────────────────────────────────────────┘
                              ↑
┌─────────────────────────────────────────────────────────────┐
│                     Hanzo Growth                            │
├─────────────────────────────────────────────────────────────┤
│  Insights      │  Analytics      │  GrowthBook   │ Dittofeed│
│  (PostHog)     │  (Umami)        │  (A/B Test)   │ (Engage) │
└─────────────────────────────────────────────────────────────┘
                              ↑
┌─────────────────────────────────────────────────────────────┐
│                    Hanzo Automation                         │
├─────────────────────────────────────────────────────────────┤
│  Activepieces  │  Temporal       │  NATS                    │
│  (Workflows)   │  (Tasks)        │  (Pub/Sub)               │
└─────────────────────────────────────────────────────────────┘
                              ↑
┌─────────────────────────────────────────────────────────────┐
│                   Hanzo CX & Ops                            │
├─────────────────────────────────────────────────────────────┤
│  Chatwoot      │  CRM            │  ERP                     │
│  (Support)     │  (Sales)        │  (Inventory)             │
└─────────────────────────────────────────────────────────────┘
```

### Deployment

```yaml
# compose.yml for production
services:
  commerce:
    image: hanzoai/commerce:latest
    environment:
      - COMMERCE_DATASTORE=native://clickhouse:9000/commerce
      - IAM_ISSUER=https://hanzo.id
      - IAM_CLIENT_ID=${IAM_CLIENT_ID}
      - INSIGHTS_ENABLED=true
      - INSIGHTS_ENDPOINT=https://insights.hanzo.ai
      - INSIGHTS_API_KEY=${INSIGHTS_API_KEY}
      - ANALYTICS_ENABLED=true
      - ANALYTICS_ENDPOINT=https://analytics.hanzo.ai
    depends_on:
      - clickhouse
      - redis
```

## Related Projects

- `~/work/hanzo/base` - Reference architecture for standalone binary
- `~/work/hanzo/datastore-go` - ClickHouse driver for Hanzo Datastore
- `~/work/hanzo/iam` - Casdoor-based IAM (hanzo.id authentication)
- `~/work/hanzo/cloud` - Casibase AI platform (100+ LLM providers)
- `~/work/hanzo/cloud-backend` - Rust inference backend with GRPO
- `~/work/hanzo/insights` - PostHog fork for product analytics
- `~/work/hanzo/analytics` - Privacy-focused web analytics (Umami-like)
- `~/work/hanzo/universe` - Production infrastructure (private)
