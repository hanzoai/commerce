// Package commerce provides the main application framework for Hanzo Commerce.
//
// Commerce is a multi-tenant e-commerce platform that runs as a standalone
// binary with embedded SQLite for per-user/org data and optional analytics
// via ClickHouse.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                     Commerce App                            │
//	├─────────────────────────────────────────────────────────────┤
//	│  HTTP Server (Gin)  │  Hooks System  │  Background Tasks    │
//	├─────────────────────────────────────────────────────────────┤
//	│  User SQLite        │  Org SQLite    │  Analytics (CH)      │
//	│  + sqlite-vec       │  + sqlite-vec  │  (parallel queries)  │
//	└─────────────────────────────────────────────────────────────┘
package commerce

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/hanzoai/commerce/auth"
	commerceDatastore "github.com/hanzoai/commerce/datastore"
	commerceQuery "github.com/hanzoai/commerce/datastore/query"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/hooks"
	"github.com/hanzoai/commerce/infra"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	planModel "github.com/hanzoai/commerce/models/deprecated/plan"
	orgModel "github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/types"
)

// Version is the current version of Commerce
const Version = "1.36.2"

// Config holds application configuration
type Config struct {
	// DataDir is the base directory for all data
	DataDir string

	// Dev enables development mode
	Dev bool

	// Secret for encryption and sessions
	Secret string

	// HTTP server address
	HTTPAddr string

	// HTTPS server address (optional)
	HTTPSAddr string

	// TLS certificate paths
	TLSCert string
	TLSKey  string

	// CORS allowed origins
	AllowedOrigins []string

	// Database configuration
	Database db.Config

	// Analytics collector endpoint (optional)
	AnalyticsEndpoint string

	// Analytics DSN (optional, for direct ClickHouse queries)
	DatastoreDSN string

	// Infrastructure configuration
	Infra infra.Config

	// Query timeout
	QueryTimeout time.Duration

	// KMS configuration for secret management
	KMS kms.Config

	// IAM configuration for hanzo.id JWT validation
	IAM struct {
		Enabled      bool   `json:"enabled"`
		Issuer       string `json:"issuer"`
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	} `json:"iam"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	cfg := &Config{
		DataDir:           getEnv("COMMERCE_DIR", "./commerce_data"),
		Dev:               getEnv("COMMERCE_DEV", "false") == "true",
		Secret:            getEnv("COMMERCE_SECRET", "change-me-in-production"),
		HTTPAddr:          getEnv("COMMERCE_HTTP", "127.0.0.1:8090"),
		AllowedOrigins:    []string{"*"},
		AnalyticsEndpoint: getEnv("ANALYTICS_ENDPOINT", ""),
		DatastoreDSN:      getEnv("DATASTORE_URL", ""),
		Infra:             *infraConfigFromEnv(),
		QueryTimeout:      30 * time.Second,
	}

	cfg.KMS.Enabled = getEnv("KMS_ENABLED", "false") == "true"
	cfg.KMS.URL = getEnv("KMS_URL", "")
	cfg.KMS.ClientID = getEnv("KMS_CLIENT_ID", "")
	cfg.KMS.ClientSecret = getEnv("KMS_CLIENT_SECRET", "")
	cfg.KMS.ProjectID = getEnv("KMS_PROJECT_ID", "")
	cfg.KMS.Environment = getEnv("KMS_ENVIRONMENT", "prod")

	cfg.IAM.Enabled = getEnv("IAM_ENABLED", "true") == "true"
	cfg.IAM.Issuer = getEnv("IAM_ISSUER", "https://hanzo.id")
	cfg.IAM.ClientID = getEnv("IAM_CLIENT_ID", "")
	cfg.IAM.ClientSecret = getEnv("IAM_CLIENT_SECRET", "")

	return cfg
}

// infraConfigFromEnv loads infrastructure config from environment.
//
// Env vars (generic, no implementation leakage):
//
//	KV_URL        = redis://:password@host:6379/0
//	S3_URL        = s3://key:secret@host:9000/bucket
//	S3_ENDPOINT   = host:9000  (with S3_ACCESS_KEY, S3_SECRET_KEY, S3_BUCKET)
//	DATASTORE_URL = clickhouse://host:9000/db
//	DOC_URL       = mongodb://host:27017/db
//	SQL_URL       = postgresql://user:pass@host:5432/db
//	VECTOR_URL    = qdrant://host:6334
//	SEARCH_URL    = http://host:7700
//	PUBSUB_URL    = nats://host:4222
//	TASKS_URL     = temporal://host:7233/namespace
func infraConfigFromEnv() *infra.Config {
	cfg := infra.DefaultConfig()

	// KV (Redis-compatible)
	if kvURL := getEnv("KV_URL", ""); kvURL != "" {
		if parsed, err := url.Parse(kvURL); err == nil {
			cfg.KV.Enabled = true
			cfg.KV.Addr = parsed.Host
			if parsed.User != nil {
				if pwd, ok := parsed.User.Password(); ok {
					cfg.KV.Password = pwd
				}
			}
			if parsed.Path != "" && parsed.Path != "/" {
				dbNum := strings.TrimPrefix(parsed.Path, "/")
				if db, err := strconv.Atoi(dbNum); err == nil {
					cfg.KV.DB = db
				}
			}
		}
	}

	// Vector (Qdrant)
	if vectorURL := getEnv("VECTOR_URL", ""); vectorURL != "" {
		if parsed, err := url.Parse(vectorURL); err == nil {
			cfg.Vector.Enabled = true
			host := parsed.Hostname()
			cfg.Vector.Host = host
			if p := parsed.Port(); p != "" {
				if port, err := strconv.Atoi(p); err == nil {
					cfg.Vector.Port = port
				}
			}
			if parsed.User != nil {
				cfg.Vector.APIKey = parsed.User.Username()
			}
		}
	}

	// Storage (S3-compatible)
	if s3URL := getEnv("S3_URL", ""); s3URL != "" {
		if parsed, err := url.Parse(s3URL); err == nil {
			cfg.Storage.Enabled = true
			cfg.Storage.Endpoint = parsed.Host
			if parsed.User != nil {
				cfg.Storage.AccessKey = parsed.User.Username()
				if pwd, ok := parsed.User.Password(); ok {
					cfg.Storage.SecretKey = pwd
				}
			}
			if bucket := strings.TrimPrefix(parsed.Path, "/"); bucket != "" {
				cfg.Storage.Bucket = bucket
			}
			cfg.Storage.UseSSL = parsed.Scheme == "s3s" || parsed.Query().Get("ssl") == "true"
		}
	} else if endpoint := getEnv("S3_ENDPOINT", ""); endpoint != "" {
		cfg.Storage.Enabled = true
		cfg.Storage.Endpoint = endpoint
		cfg.Storage.AccessKey = getEnv("S3_ACCESS_KEY", "")
		cfg.Storage.SecretKey = getEnv("S3_SECRET_KEY", "")
		cfg.Storage.Bucket = getEnv("S3_BUCKET", "commerce")
		cfg.Storage.UseSSL = getEnv("S3_USE_SSL", "false") == "true"
	}

	// Search (Meilisearch)
	if searchURL := getEnv("SEARCH_URL", ""); searchURL != "" {
		cfg.Search.Enabled = true
		cfg.Search.Host = searchURL
		if parsed, err := url.Parse(searchURL); err == nil {
			if parsed.User != nil {
				cfg.Search.APIKey = parsed.User.Username()
			}
		}
	}

	// PubSub (NATS)
	if pubsubURL := getEnv("PUBSUB_URL", ""); pubsubURL != "" {
		cfg.PubSub.Enabled = true
		cfg.PubSub.URL = pubsubURL
		if parsed, err := url.Parse(pubsubURL); err == nil {
			if parsed.User != nil {
				cfg.PubSub.Token = parsed.User.Username()
			}
		}
		cfg.PubSub.EnableJetStream = getEnv("PUBSUB_JETSTREAM", "true") == "true"
	}

	// Tasks (Temporal)
	if tasksURL := getEnv("TASKS_URL", ""); tasksURL != "" {
		if parsed, err := url.Parse(tasksURL); err == nil {
			cfg.Tasks.Enabled = true
			cfg.Tasks.HostPort = parsed.Host
			if ns := strings.TrimPrefix(parsed.Path, "/"); ns != "" {
				cfg.Tasks.Namespace = ns
			}
		}
	}

	return cfg
}

// App is the main Commerce application
type App struct {
	config *Config

	// Root command
	RootCmd *cobra.Command

	// Database manager
	DB *db.Manager

	// Infrastructure manager
	Infra *infra.Manager

	// Hook system
	Hooks *hooks.Registry

	// Events client (sends to analytics-collector via HTTP)
	Events *events.Client

	// KMS client for secret management
	KMS *kms.CachedClient

	// HTTP router
	Router *gin.Engine

	// HTTP server
	server *http.Server

	// Shutdown handling
	shutdownOnce sync.Once
	shutdownCh   chan struct{}

	// State
	bootstrapped bool
	mu           sync.RWMutex
}

// New creates a new Commerce application with default configuration
func New() *App {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a new Commerce application with the given configuration
func NewWithConfig(config *Config) *App {
	app := &App{
		config:     config,
		Hooks:      hooks.NewRegistry(),
		shutdownCh: make(chan struct{}),
	}

	// Set Gin mode
	if config.Dev {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize CLI
	app.initCLI()

	return app
}

// initCLI sets up the cobra command structure
func (app *App) initCLI() {
	app.RootCmd = &cobra.Command{
		Use:     "commerce",
		Short:   "Hanzo Commerce - Multi-tenant e-commerce platform",
		Version: Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip bootstrap for help/version
			if cmd.Name() == "help" || cmd.Name() == "version" {
				return nil
			}
			return app.Bootstrap()
		},
	}

	// Global flags
	app.RootCmd.PersistentFlags().StringVar(&app.config.DataDir, "dir", app.config.DataDir, "Data directory")
	app.RootCmd.PersistentFlags().BoolVar(&app.config.Dev, "dev", app.config.Dev, "Enable development mode")
	app.RootCmd.PersistentFlags().StringVar(&app.config.Secret, "secret", app.config.Secret, "Encryption secret")

	// Add commands
	app.RootCmd.AddCommand(app.newServeCmd())
	app.RootCmd.AddCommand(app.newMigrateCmd())
	app.RootCmd.AddCommand(app.newAdminCmd())
	app.RootCmd.AddCommand(app.newSeedCmd())
}

// newServeCmd creates the serve command
func (app *App) newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve [address]",
		Short: "Start the Commerce server",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				app.config.HTTPAddr = args[0]
			}
			return app.Serve()
		},
	}

	cmd.Flags().StringSliceVar(&app.config.AllowedOrigins, "origins", app.config.AllowedOrigins, "CORS allowed origins")
	cmd.Flags().StringVar(&app.config.HTTPSAddr, "https", "", "HTTPS address")
	cmd.Flags().StringVar(&app.config.TLSCert, "cert", "", "TLS certificate path")
	cmd.Flags().StringVar(&app.config.TLSKey, "key", "", "TLS key path")

	return cmd
}

// newMigrateCmd creates the migrate command
func (app *App) newMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Running migrations...")
			// TODO: Implement migration system
			return nil
		},
	}
}

// newAdminCmd creates the admin command
func (app *App) newAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Admin user management",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "create [email]",
		Short: "Create an admin user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			email := args[0]
			fmt.Printf("Creating admin user: %s\n", email)
			// TODO: Implement admin creation
			return nil
		},
	})

	return cmd
}

// newSeedCmd creates the seed command for bootstrapping organizations and plans
func (app *App) newSeedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed [org-name]",
		Short: "Seed organization and plans for a service",
		Long:  "Bootstrap an organization with API tokens and subscription plans.\nDefault org: bootnode",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			orgName := "bootnode"
			if len(args) > 0 {
				orgName = args[0]
			}
			return app.seedOrganization(orgName)
		},
	}
	return cmd
}

func (app *App) seedOrganization(orgName string) error {
	ctx := context.Background()
	ds := commerceDatastore.New(ctx)

	// Create or get organization
	org := orgModel.New(ds)
	org.Name = orgName
	org.GetOrCreate("Name=", org.Name)
	org.FullName = orgName + " Platform"
	org.Enabled = true
	org.AddDefaultTokens()

	org.MustPut()

	// Write payment credentials to KMS (if enabled)
	if app.KMS != nil {
		client := app.KMS.Client()
		stripePath := "/tenants/" + orgName + "/stripe"
		squarePath := "/tenants/" + orgName + "/square"

		seedSecrets := []struct{ path, name, envVar string }{
			// Stripe
			{stripePath, "STRIPE_LIVE_ACCESS_TOKEN", "STRIPE_SECRET_KEY"},
			{stripePath, "STRIPE_TEST_ACCESS_TOKEN", "STRIPE_TEST_SECRET_KEY"},
			{stripePath, "STRIPE_PUBLISHABLE_KEY", "STRIPE_PUBLISHABLE_KEY"},
			// Square — Production
			{squarePath, "SQUARE_PRODUCTION_APPLICATION_ID", "SQUARE_APPLICATION_ID"},
			{squarePath, "SQUARE_PRODUCTION_ACCESS_TOKEN", "SQUARE_ACCESS_TOKEN"},
			{squarePath, "SQUARE_PRODUCTION_LOCATION_ID", "SQUARE_LOCATION_ID"},
			// Square — Sandbox
			{squarePath, "SQUARE_SANDBOX_APPLICATION_ID", "SQUARE_SANDBOX_APPLICATION_ID"},
			{squarePath, "SQUARE_SANDBOX_ACCESS_TOKEN", "SQUARE_SANDBOX_ACCESS_TOKEN"},
			{squarePath, "SQUARE_SANDBOX_LOCATION_ID", "SQUARE_SANDBOX_LOCATION_ID"},
			// Square — Webhook
			{squarePath, "SQUARE_WEBHOOK_SIGNATURE_KEY", "SQUARE_WEBHOOK_SIGNATURE_KEY"},
		}

		for _, s := range seedSecrets {
			if v := os.Getenv(s.envVar); v != "" {
				if err := client.SetSecret(s.path, s.name, v); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to write %s to KMS: %v\n", s.name, err)
				} else {
					fmt.Printf("KMS: wrote %s to %s\n", s.name, s.path)
				}
			}
		}
	}

	// Get the test-secret-key token for API access
	tok, err := org.GetTokenByName("test-secret-key")
	if err != nil {
		return fmt.Errorf("failed to get API token: %w", err)
	}

	fmt.Printf("Organization: %s (ID: %s)\n", org.Name, org.Id())
	fmt.Printf("API Key (test): %s\n", tok.String)

	liveTok, err := org.GetTokenByName("live-secret-key")
	if err == nil {
		fmt.Printf("API Key (live): %s\n", liveTok.String)
	}

	// Create plans in the org's namespace
	nsDs := commerceDatastore.New(ctx)
	nsDs.SetNamespace(org.Namespace())

	plans := []struct {
		slug        string
		name        string
		price       int64
		interval    string
		description string
	}{
		{"bootnode-free", "Bootnode Free", 0, "month", "Free tier: 30M CU/mo, 25 req/s"},
		{"bootnode-payg", "Bootnode Pay-As-You-Go", 0, "month", "PAYG: $0.40/M CU, 300 req/s"},
		{"bootnode-growth", "Bootnode Growth", 4900, "month", "Growth: 100M CU included, $0.35/M overage, 500 req/s"},
		{"bootnode-enterprise", "Bootnode Enterprise", 0, "month", "Enterprise: custom pricing, 1000+ req/s"},
	}

	for _, p := range plans {
		pln := planModel.New(nsDs)
		pln.Slug = p.slug
		pln.GetOrCreate("Slug=", pln.Slug)
		pln.Name = p.name
		pln.Price = currency.Cents(p.price)
		pln.Currency = currency.USD
		pln.Interval = types.Monthly
		pln.IntervalCount = 1
		pln.Description = p.description
		pln.MustPut()
		fmt.Printf("Plan: %s (%s) - $%.2f/mo\n", pln.Name, pln.Slug, float64(pln.Price)/100)
	}

	fmt.Println("\nSeed complete.")
	return nil
}

// Start runs the application
func (app *App) Start() error {
	return app.RootCmd.Execute()
}

// Bootstrap initializes the application
func (app *App) Bootstrap() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.bootstrapped {
		return nil
	}

	// Trigger OnBootstrap hooks
	if err := app.Hooks.TriggerBootstrap(app); err != nil {
		return fmt.Errorf("bootstrap hook error: %w", err)
	}

	// Ensure data directory exists
	if err := os.MkdirAll(app.config.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Initialize database manager
	dbConfig := &db.Config{
		DataDir:            app.config.DataDir,
		DatastoreDSN:       app.config.DatastoreDSN,
		EnableDatastore:    app.config.DatastoreDSN != "",
		EnableVectorSearch: true,
		VectorDimensions:   1536,
		IsDev:              app.config.Dev,
	}

	var err error
	app.DB, err = db.NewManager(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Set up system-level SQLite DB as the default for all datastore operations.
	// This bridges the legacy GAE datastore API to the new SQLite backend.
	systemDB, err := app.DB.Org("system")
	if err != nil {
		return fmt.Errorf("failed to initialize system database: %w", err)
	}
	commerceDatastore.SetDefaultDB(systemDB)
	commerceQuery.SetDefaultDB(systemDB)

	// Initialize infrastructure manager
	app.Infra = infra.New(&app.config.Infra)
	ctx, cancel := context.WithTimeout(context.Background(), app.config.Infra.ConnectTimeout)
	defer cancel()

	if err := app.Infra.Connect(ctx); err != nil {
		// Log but don't fail - infrastructure services are optional
		fmt.Fprintf(os.Stderr, "Warning: some infrastructure services unavailable: %v\n", err)
	}

	// Initialize KMS client for secret management
	if app.config.KMS.Enabled && app.config.KMS.URL != "" {
		kmsClient := kms.NewClient(&app.config.KMS)
		app.KMS = kms.NewCachedClient(kmsClient)
		fmt.Println("KMS client initialized")
	}

	// Initialize analytics client (sends events to analytics-collector via HTTP)
	if app.config.AnalyticsEndpoint != "" {
		app.Events = events.NewClient(app.config.AnalyticsEndpoint)
	}

	// Initialize router
	app.Router = gin.New()
	app.Router.Use(gin.Recovery())
	if app.config.Dev {
		app.Router.Use(gin.Logger())
	}

	// Initialize IAM middleware for hanzo.id JWT validation
	if app.config.IAM.Enabled && app.config.IAM.Issuer != "" && app.config.IAM.ClientID != "" {
		iamCfg := &auth.IAMConfig{
			Issuer:       app.config.IAM.Issuer,
			ClientID:     app.config.IAM.ClientID,
			ClientSecret: app.config.IAM.ClientSecret,
		}
		if err := iammiddleware.Init(iamCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: IAM middleware initialization failed: %v\n", err)
			app.config.IAM.Enabled = false
		}
	} else if app.config.IAM.ClientID == "" {
		app.config.IAM.Enabled = false
	}

	// Setup routes
	app.setupRoutes()

	app.bootstrapped = true
	return nil
}

// setupRoutes configures HTTP routes
func (app *App) setupRoutes() {
	// Health check
	app.Router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": Version,
		})
	})

	// API routes
	api := app.Router.Group("/api/v1")
	{
		// Core middleware required by Commerce API handlers
		api.Use(middleware.AddHost())
		api.Use(middleware.RequestContext())
		api.Use(middleware.DetectOverrides())
		api.Use(middleware.ErrorHandlerJSON())
		api.Use(middleware.AccessControl("*"))

		// IAM JWT validation middleware (falls through to legacy auth if not IAM token)
		if app.config.IAM.Enabled {
			api.Use(iammiddleware.IAMTokenRequired())
		}

		// Inject KMS and Events into gin context for handlers
		api.Use(func(c *gin.Context) {
			if app.KMS != nil {
				c.Set("kms", app.KMS)
			}
			if app.Events != nil {
				c.Set("events", app.Events)
			}
			c.Next()
		})

		// Trigger OnRouteSetup hooks to let extensions add routes
		app.Hooks.TriggerRouteSetup(api)
	}
}

// Serve starts the HTTP server
func (app *App) Serve() error {
	// Trigger OnServe hooks
	if err := app.Hooks.TriggerServe(app); err != nil {
		return fmt.Errorf("serve hook error: %w", err)
	}

	app.server = &http.Server{
		Addr:         app.config.HTTPAddr,
		Handler:      app.Router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	fmt.Printf("Commerce %s starting on %s\n", Version, app.config.HTTPAddr)
	if app.config.Dev {
		fmt.Println("Running in DEVELOPMENT mode")
	}

	// Start HTTPS if configured
	if app.config.HTTPSAddr != "" && app.config.TLSCert != "" && app.config.TLSKey != "" {
		go func() {
			httpsServer := &http.Server{
				Addr:         app.config.HTTPSAddr,
				Handler:      app.Router,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				IdleTimeout:  120 * time.Second,
			}
			if err := httpsServer.ListenAndServeTLS(app.config.TLSCert, app.config.TLSKey); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "HTTPS error: %v\n", err)
			}
		}()
	}

	// Start HTTP server
	if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// Shutdown gracefully shuts down the application
func (app *App) Shutdown() error {
	var err error
	app.shutdownOnce.Do(func() {
		close(app.shutdownCh)

		// Trigger OnTerminate hooks
		if hookErr := app.Hooks.TriggerTerminate(app); hookErr != nil {
			err = hookErr
		}

		// Shutdown HTTP server
		if app.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if shutdownErr := app.server.Shutdown(ctx); shutdownErr != nil {
				err = shutdownErr
			}
		}

		// Close events emitter (flush remaining events)
		if app.Events != nil {
			if eventsErr := app.Events.Close(); eventsErr != nil {
				err = eventsErr
			}
		}

		// Close infrastructure
		if app.Infra != nil {
			if infraErr := app.Infra.Close(); infraErr != nil {
				err = infraErr
			}
		}

		// Close database
		if app.DB != nil {
			if dbErr := app.DB.Close(); dbErr != nil {
				err = dbErr
			}
		}
	})

	return err
}

// Config returns the current configuration
func (app *App) Config() *Config {
	return app.config
}

// DataPath returns the full path within the data directory
func (app *App) DataPath(subpath string) string {
	return filepath.Join(app.config.DataDir, subpath)
}

// IsDev returns true if running in development mode
func (app *App) IsDev() bool {
	return app.config.Dev
}

// getEnv returns environment variable or default
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
