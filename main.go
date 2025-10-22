package main

import (
"flag"
"fmt"
"html/template"
"io/fs"
"log"
"os"
"os/signal"
"syscall"
"time"

"claude-code-codex-companion/internal/common/httpclient"
"claude-code-codex-companion/internal/config"
"claude-code-codex-companion/internal/database"
"claude-code-codex-companion/internal/proxy"
	"claude-code-codex-companion/internal/webres"
)

var (
	configFile         = flag.String("config", "config.yaml", "Configuration file path")
	port               = flag.Int("port", 0, "Override proxy server port")
	version            = flag.Bool("version", false, "Show version information")
	migrateFromPython = flag.String("migrate-from-python", "", "Migrate from Python Api-Conversion database (provide path to old SQLite database)")

	// This will be set by build process
	Version = "dev"
)


// EmbeddedAssetProvider implements webres.AssetProvider using embedded assets
type EmbeddedAssetProvider struct{}

// NewEmbeddedAssetProvider creates a new provider
func NewEmbeddedAssetProvider() *EmbeddedAssetProvider {
	return &EmbeddedAssetProvider{}
}

// GetTemplateFS returns the embedded template filesystem
func (p *EmbeddedAssetProvider) GetTemplateFS() (fs.FS, error) {
	return os.DirFS("web/templates"), nil
}

// GetStaticFS returns the embedded static filesystem
func (p *EmbeddedAssetProvider) GetStaticFS() (fs.FS, error) {
	return os.DirFS("web/static"), nil
}

// GetLocalesFS returns the embedded locales filesystem
func (p *EmbeddedAssetProvider) GetLocalesFS() (fs.FS, error) {
	return os.DirFS("web/locales"), nil
}

// LoadTemplates loads all templates from embedded filesystem
func (p *EmbeddedAssetProvider) LoadTemplates() (*template.Template, error) {
	templateFS, err := p.GetTemplateFS()
	if err != nil {
		return nil, err
	}
	
	return template.ParseFS(templateFS, "*.html")
}

// ReadLocaleFile reads a locale file from embedded filesystem
func (p *EmbeddedAssetProvider) ReadLocaleFile(filename string) ([]byte, error) {
	localesFS, err := p.GetLocalesFS()
	if err != nil {
		return nil, err
	}
	
	return fs.ReadFile(localesFS, filename)
}

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("Claude Code Codex Companion %s\n", Version)
		os.Exit(0)
	}

	// Handle database migration from Python Api-Conversion
	if *migrateFromPython != "" {
		fmt.Printf("Starting database migration from Python Api-Conversion...\n")
		newDBPath := "cccc.db" // Default database file
		if err := database.MigrateFromPython(*migrateFromPython, newDBPath, ""); err != nil {
			log.Fatalf("Database migration failed: %v", err)
		}
		fmt.Printf("Database migration completed successfully!\n")
		fmt.Printf("New database created at: %s\n", newDBPath)
		fmt.Printf("You can now start the server normally.\n")
		os.Exit(0)
	}

	// Initialize embedded web assets
	webres.SetProvider(NewEmbeddedAssetProvider())

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if *port > 0 {
		cfg.Server.Port = *port
	}

	// Initialize HTTP clients with configured timeouts
	if err := initHTTPClientsFromConfig(cfg); err != nil {
		log.Fatalf("Failed to initialize HTTP clients: %v", err)
	}
	proxyServer, err := proxy.NewServer(cfg, *configFile, Version)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	go func() {
		log.Printf("Starting proxy server on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := proxyServer.Start(); err != nil {
			log.Fatalf("Proxy server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("\n=== Claude Code Codex Companion %s ===\n", Version)
	fmt.Printf("Proxy Server: http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("Admin Interface: http://%s:%d/admin/\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("Configuration File: %s\n", *configFile)
	fmt.Printf("\nPress Ctrl+C to stop the server...\n\n")

	<-quit
	fmt.Println("\nShutting down servers...")
	
	// Graceful shutdown: close logger and database connections
	if logger := proxyServer.GetLogger(); logger != nil {
		if err := logger.Close(); err != nil {
			log.Printf("Error closing logger: %v", err)
		} else {
			log.Println("Logger closed successfully")
		}
	}
}

// initHTTPClientsFromConfig initializes HTTP clients with timeout configurations
func initHTTPClientsFromConfig(cfg *config.Config) error {
	// Parse proxy timeouts
	proxyTimeouts := httpclient.TimeoutConfig{}
	
	var err error
	if proxyTimeouts.TLSHandshake, err = httpclient.ParseTimeoutWithDefault(cfg.Timeouts.TLSHandshake, "tls_handshake", config.GetTimeoutDuration(config.Default.Timeouts.TLSHandshake, 10*time.Second)); err != nil {
		return err
	}
	
	if proxyTimeouts.ResponseHeader, err = httpclient.ParseTimeoutWithDefault(cfg.Timeouts.ResponseHeader, "response_header", config.GetTimeoutDuration(config.Default.Timeouts.ResponseHeader, 60*time.Second)); err != nil {
		return err
	}
	
	if proxyTimeouts.IdleConnection, err = httpclient.ParseTimeoutWithDefault(cfg.Timeouts.IdleConnection, "idle_connection", config.GetTimeoutDuration(config.Default.Timeouts.IdleConnection, 90*time.Second)); err != nil {
		return err
	}
	
	// Overall request timeout is optional for proxy (streaming support)
	if proxyTimeouts.OverallRequest, err = httpclient.ParseTimeoutWithDefault("", "overall_request", 0); err != nil {
		return err
	}
	
	// Parse health check timeouts
	healthTimeouts := httpclient.TimeoutConfig{}
	
	if healthTimeouts.TLSHandshake, err = httpclient.ParseTimeoutWithDefault(cfg.Timeouts.TLSHandshake, "health_check.tls_handshake", config.GetTimeoutDuration(config.Default.Timeouts.TLSHandshake, 10*time.Second)); err != nil {
		return err
	}
	
	if healthTimeouts.ResponseHeader, err = httpclient.ParseTimeoutWithDefault(cfg.Timeouts.ResponseHeader, "health_check.response_header", config.GetTimeoutDuration(config.Default.Timeouts.ResponseHeader, 60*time.Second)); err != nil {
		return err
	}
	
	if healthTimeouts.IdleConnection, err = httpclient.ParseTimeoutWithDefault(cfg.Timeouts.IdleConnection, "health_check.idle_connection", config.GetTimeoutDuration(config.Default.Timeouts.IdleConnection, 90*time.Second)); err != nil {
		return err
	}
	
	if healthTimeouts.OverallRequest, err = httpclient.ParseTimeoutWithDefault(cfg.Timeouts.HealthCheckTimeout, "health_check.overall_request", config.GetTimeoutDuration(config.Default.Timeouts.HealthCheckTimeout, 30*time.Second)); err != nil {
		return err
	}
	
	// Initialize HTTP clients
	httpclient.InitHTTPClients(proxyTimeouts, healthTimeouts)
	
	return nil
}