# Commerce - LLM Context Document

## Overview

Hanzo Commerce is a multi-tenant e-commerce platform being modernized from a Google App Engine monolith to a standalone binary with embedded SQLite.

## Architecture

### Current State (Modernization In Progress)

```
┌─────────────────────────────────────────────────────────────┐
│                     Commerce App                            │
├─────────────────────────────────────────────────────────────┤
│  CLI (cobra)    │  HTTP (Gin)    │  Hooks System            │
├─────────────────────────────────────────────────────────────┤
│  User SQLite    │  Org SQLite    │  Hanzo Datastore         │
│  + sqlite-vec   │  + sqlite-vec  │  (ClickHouse)            │
│  Per-user data  │  Shared tenant │  Deep analytics          │
└─────────────────────────────────────────────────────────────┘
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
├── cmd/commerce/       # CLI entry point (NEW)
├── commerce.go         # Main app framework (NEW)
├── db/                 # Database abstraction (NEW)
│   ├── db.go           # Interfaces and Manager
│   ├── sqlite.go       # SQLite implementation
│   ├── postgres.go     # PostgreSQL implementation
│   ├── mongo.go        # MongoDB/FerretDB implementation
│   ├── query.go        # Query builder
│   └── datastore.go    # Hanzo Datastore (ClickHouse) connector
├── hooks/              # Hook system (NEW)
│   └── hooks.go        # Event hooks for extensibility
├── api/                # HTTP API handlers (legacy)
├── models/             # Data models (legacy, needs migration)
├── datastore/          # Old App Engine datastore (legacy)
├── config/             # Configuration (legacy)
└── middleware/         # HTTP middleware (legacy)
```

## Running Commerce

### As Standalone Binary

```bash
# Development
go run cmd/commerce/main.go serve --dev

# Production
./commerce serve 0.0.0.0:80

# With Hanzo Datastore
COMMERCE_DATASTORE="native://localhost:9000/commerce" ./commerce serve
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `COMMERCE_DIR` | Data directory | `./commerce_data` |
| `COMMERCE_DEV` | Development mode | `false` |
| `COMMERCE_SECRET` | Encryption secret | `change-me-in-production` |
| `COMMERCE_HTTP` | HTTP address | `127.0.0.1:8090` |
| `COMMERCE_DATASTORE` | Hanzo Datastore DSN | (disabled) |

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

## Hook System

### Registering Hooks

```go
// On order creation
app.Hooks.OnModelCreate("Order").Bind(&hooks.Handler[*hooks.ModelEvent]{
    ID:       "validateInventory",
    Priority: 10,
    Func: func(e *hooks.ModelEvent) error {
        order := e.Model.(*Order)
        // Validate inventory
        return e.Next()
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

## Legacy Code Notes

### App Engine Dependencies (To Be Migrated)

The following packages still depend on `google.golang.org/appengine`:

- `datastore/` - Core datastore wrapper
- `models/mixin/` - Model base class
- `middleware/` - HTTP middleware
- `api/` - API handlers
- `delay/` - Task queue

### Migration Strategy

1. **Phase 1** (Complete): Create new `db/` package with SQLite backend
2. **Phase 2** (In Progress): Create standalone CLI and app framework
3. **Phase 3** (Pending): Migrate models to new `db.DB` interface
4. **Phase 4** (Pending): Remove App Engine dependencies
5. **Phase 5** (Pending): Integrate `hanzo/iam` for auth

## Dependencies

### New (Modernization)

- `github.com/spf13/cobra` - CLI framework
- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/gin-gonic/gin` - HTTP framework (existing)

### Legacy (To Be Evaluated)

- `google.golang.org/appengine` - App Engine SDK (to be removed)
- `github.com/qedus/nds` - Datastore caching (to be replaced)

## Testing

```bash
# Run all tests
go test ./...

# Run specific package
go test ./db/...

# With verbose output
go test -v ./...
```

## Security Considerations

1. **Secrets**: All secrets removed from codebase, use environment variables
2. **Per-user isolation**: SQLite databases are isolated per user
3. **Vector embeddings**: Stored locally, no external API calls for search

## Related Projects

- `~/work/hanzo/base` - Reference architecture for standalone binary
- `~/work/hanzo/datastore-go` - ClickHouse driver for analytics
- `~/work/hanzo/iam` - Authentication service (to be integrated)
- `~/work/hanzo/analytics` - Analytics service (to be integrated)
