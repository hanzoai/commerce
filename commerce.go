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
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/hooks"
	"github.com/hanzoai/commerce/infra"
)

// Version is the current version of Commerce
const Version = "2.0.0"

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

	// Analytics DSN (optional)
	DatastoreDSN string

	// Infrastructure configuration
	Infra infra.Config

	// Query timeout
	QueryTimeout time.Duration
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		DataDir:        getEnv("COMMERCE_DIR", "./commerce_data"),
		Dev:            getEnv("COMMERCE_DEV", "false") == "true",
		Secret:         getEnv("COMMERCE_SECRET", "change-me-in-production"),
		HTTPAddr:       getEnv("COMMERCE_HTTP", "127.0.0.1:8090"),
		AllowedOrigins: []string{"*"},
		DatastoreDSN:   getEnv("COMMERCE_DATASTORE", ""),
		Infra:          *infraConfigFromEnv(),
		QueryTimeout:   30 * time.Second,
	}
}

// infraConfigFromEnv loads infrastructure config from environment
func infraConfigFromEnv() *infra.Config {
	cfg := infra.DefaultConfig()

	// KV (Valkey)
	if addr := getEnv("VALKEY_ADDR", ""); addr != "" {
		cfg.KV.Enabled = true
		cfg.KV.Addr = addr
		cfg.KV.Password = getEnv("VALKEY_PASSWORD", "")
	}

	// Vector (Qdrant)
	if host := getEnv("QDRANT_HOST", ""); host != "" {
		cfg.Vector.Enabled = true
		cfg.Vector.Host = host
		if port, err := strconv.Atoi(getEnv("QDRANT_PORT", "6334")); err == nil {
			cfg.Vector.Port = port
		}
		cfg.Vector.APIKey = getEnv("QDRANT_API_KEY", "")
	}

	// Storage (MinIO)
	if endpoint := getEnv("MINIO_ENDPOINT", ""); endpoint != "" {
		cfg.Storage.Enabled = true
		cfg.Storage.Endpoint = endpoint
		cfg.Storage.AccessKey = getEnv("MINIO_ACCESS_KEY", "minioadmin")
		cfg.Storage.SecretKey = getEnv("MINIO_SECRET_KEY", "minioadmin")
		cfg.Storage.Bucket = getEnv("MINIO_BUCKET", "commerce")
		cfg.Storage.UseSSL = getEnv("MINIO_USE_SSL", "false") == "true"
	}

	// Search (Meilisearch)
	if host := getEnv("MEILISEARCH_HOST", ""); host != "" {
		cfg.Search.Enabled = true
		cfg.Search.Host = host
		cfg.Search.APIKey = getEnv("MEILISEARCH_API_KEY", "")
	}

	// PubSub (NATS)
	if url := getEnv("NATS_URL", ""); url != "" {
		cfg.PubSub.Enabled = true
		cfg.PubSub.URL = url
		cfg.PubSub.Token = getEnv("NATS_TOKEN", "")
		cfg.PubSub.EnableJetStream = getEnv("NATS_JETSTREAM", "true") == "true"
	}

	// Tasks (Temporal)
	if host := getEnv("TEMPORAL_HOST", ""); host != "" {
		cfg.Tasks.Enabled = true
		cfg.Tasks.HostPort = host
		cfg.Tasks.Namespace = getEnv("TEMPORAL_NAMESPACE", "commerce")
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

	// Initialize infrastructure manager
	app.Infra = infra.New(&app.config.Infra)
	ctx, cancel := context.WithTimeout(context.Background(), app.config.Infra.ConnectTimeout)
	defer cancel()

	if err := app.Infra.Connect(ctx); err != nil {
		// Log but don't fail - infrastructure services are optional
		fmt.Fprintf(os.Stderr, "Warning: some infrastructure services unavailable: %v\n", err)
	}

	// Initialize router
	app.Router = gin.New()
	app.Router.Use(gin.Recovery())
	if app.config.Dev {
		app.Router.Use(gin.Logger())
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
