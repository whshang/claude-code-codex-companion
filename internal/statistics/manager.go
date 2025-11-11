package statistics

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/driver/sqlite"
	_ "modernc.org/sqlite" // Pure Go SQLite driver, no CGO required
)

// Manager manages endpoint statistics persistence using SQLite
type Manager struct {
	db     *gorm.DB
	dbPath string
}

// NewManager creates a new statistics manager with independent statistics.db
func NewManager(dataDirectory string) (*Manager, error) {
	if dataDirectory == "" {
		dataDirectory = "."
	}

	dbPath := filepath.Join(dataDirectory, "statistics.db")
	
	// Configure GORM with performance optimizations for statistics
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Reduce logging for performance
		NowFunc: func() time.Time {
			return time.Now().UTC() // Use UTC for consistency
		},
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	// Use pure Go SQLite driver (modernc.org/sqlite) to avoid CGO dependency
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open statistics database: %v", err)
	}
	
	db, err := gorm.Open(sqlite.Dialector{
		Conn: sqlDB,
	}, gormConfig)
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize GORM with statistics database: %v", err)
	}

	// Configure SQLite for optimal performance
	// sqlDB is already available from the earlier sql.Open call

	// SQLite performance settings for statistics workload
	sqlDB.SetMaxOpenConns(1) // SQLite benefits from single connection
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Execute SQLite pragmas for better performance
	pragmas := []string{
		"PRAGMA journal_mode=WAL",           // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous=NORMAL",         // Balance between safety and performance
		"PRAGMA cache_size=10000",           // Increase cache size
		"PRAGMA temp_store=memory",          // Use memory for temporary tables
		"PRAGMA mmap_size=268435456",        // Use memory mapping (256MB)
		"PRAGMA optimize",                   // Optimize database
	}

	for _, pragma := range pragmas {
		if err := db.Exec(pragma).Error; err != nil {
			return nil, fmt.Errorf("failed to execute pragma %s: %v", pragma, err)
		}
	}

	// Auto-migrate the statistics table
	if err := db.AutoMigrate(&EndpointStatistics{}); err != nil {
		return nil, fmt.Errorf("failed to migrate statistics database: %v", err)
	}

	// Create additional indexes for better query performance
	if err := createAdditionalIndexes(db); err != nil {
		return nil, fmt.Errorf("failed to create additional indexes: %v", err)
	}

	manager := &Manager{
		db:     db,
		dbPath: dbPath,
	}

	return manager, nil
}

// createAdditionalIndexes creates performance indexes not handled by GORM tags
func createAdditionalIndexes(db *gorm.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_endpoint_stats_name_type ON endpoint_statistics(name, endpoint_type)",
		"CREATE INDEX IF NOT EXISTS idx_endpoint_stats_updated ON endpoint_statistics(last_updated DESC)",
		"CREATE INDEX IF NOT EXISTS idx_endpoint_stats_requests ON endpoint_statistics(total_requests DESC)",
	}

	for _, idx := range indexes {
		if err := db.Exec(idx).Error; err != nil {
			return fmt.Errorf("failed to create index: %v", err)
		}
	}

	return nil
}

// GenerateEndpointID generates a stable ID based on endpoint name
// This ensures that endpoints with the same name will have the same ID
// even across configuration changes
func GenerateEndpointID(endpointName string) string {
	nameHash := sha256.Sum256([]byte(endpointName))
	return fmt.Sprintf("ep-name-%x", nameHash[:12]) // Use first 12 bytes (24 hex chars)
}

// LoadStatistics loads statistics for a specific endpoint ID
func (m *Manager) LoadStatistics(endpointID string) (*EndpointStatistics, error) {
	var stats EndpointStatistics
	err := m.db.Where("id = ?", endpointID).First(&stats).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found is not an error, just return nil
		}
		return nil, fmt.Errorf("failed to load statistics for endpoint %s: %v", endpointID, err)
	}
	return &stats, nil
}

// LoadStatisticsByName loads statistics for an endpoint by name
// This is used during endpoint initialization to find existing statistics
func (m *Manager) LoadStatisticsByName(endpointName string) (*EndpointStatistics, error) {
	endpointID := GenerateEndpointID(endpointName)
	return m.LoadStatistics(endpointID)
}

// SaveStatistics saves or updates endpoint statistics
func (m *Manager) SaveStatistics(stats *EndpointStatistics) error {
	stats.LastUpdated = time.Now().UTC()
	
	// Use GORM's Save method which handles both create and update
	err := m.db.Save(stats).Error
	if err != nil {
		return fmt.Errorf("failed to save statistics for endpoint %s: %v", stats.ID, err)
	}
	return nil
}

// RecordRequest records a request result and updates statistics
func (m *Manager) RecordRequest(endpointID string, success bool) error {
	// Use a transaction to ensure consistency
	return m.db.Transaction(func(tx *gorm.DB) error {
		var stats EndpointStatistics
		
		// Load current statistics
		err := tx.Where("id = ?", endpointID).First(&stats).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("endpoint statistics not found for ID %s", endpointID)
			}
			return fmt.Errorf("failed to load statistics: %v", err)
		}

		// Update statistics based on request result
		stats.TotalRequests++
		stats.LastUpdated = time.Now().UTC()

		if success {
			stats.SuccessRequests++
			stats.FailureCount = 0 // Reset consecutive failures
			stats.SuccessiveSuccesses++
		} else {
			stats.FailureCount++
			stats.SuccessiveSuccesses = 0 // Reset consecutive successes
			stats.LastFailure = time.Now().UTC()
		}

		// Save updated statistics
		return tx.Save(&stats).Error
	})
}

// InitializeEndpointStatistics creates new statistics record for an endpoint
// or returns existing one if it already exists
func (m *Manager) InitializeEndpointStatistics(endpointName, url, endpointType, authType string) (*EndpointStatistics, error) {
	endpointID := GenerateEndpointID(endpointName)

	// Check if statistics already exist
	existing, err := m.LoadStatistics(endpointID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		// Update metadata in existing record
		existing.URL = url
		existing.EndpointType = endpointType
		existing.AuthType = authType
		existing.LastUpdated = time.Now().UTC()
		
		if err := m.SaveStatistics(existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// Create new statistics record
	now := time.Now().UTC()
	stats := &EndpointStatistics{
		ID:                  endpointID,
		Name:                endpointName,
		URL:                 url,
		EndpointType:        endpointType,
		AuthType:            authType,
		TotalRequests:       0,
		SuccessRequests:     0,
		FailureCount:        0,
		SuccessiveSuccesses: 0,
		LastUpdated:         now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := m.SaveStatistics(stats); err != nil {
		return nil, err
	}

	return stats, nil
}

// UpdateEndpointMetadata updates the metadata fields of an endpoint's statistics
// This is called when endpoint configuration changes but name remains the same
func (m *Manager) UpdateEndpointMetadata(endpointID, name, url, endpointType, authType string) error {
	return m.db.Model(&EndpointStatistics{}).
		Where("id = ?", endpointID).
		Updates(map[string]interface{}{
			"name":          name,
			"url":           url,
			"endpoint_type": endpointType,
			"auth_type":     authType,
			"last_updated":  time.Now().UTC(),
		}).Error
}

// DeleteStatistics removes statistics record for an endpoint
func (m *Manager) DeleteStatistics(endpointID string) error {
	result := m.db.Where("id = ?", endpointID).Delete(&EndpointStatistics{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete statistics for endpoint %s: %v", endpointID, result.Error)
	}
	return nil
}

// GetAllStatistics returns all endpoint statistics
func (m *Manager) GetAllStatistics() ([]*EndpointStatistics, error) {
	var allStats []*EndpointStatistics
	err := m.db.Order("name").Find(&allStats).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load all statistics: %v", err)
	}
	return allStats, nil
}

// GetStatisticsSummary returns a summary of all statistics
func (m *Manager) GetStatisticsSummary() (map[string]interface{}, error) {
	var result struct {
		TotalEndpoints int `gorm:"column:total_endpoints"`
		TotalRequests  int `gorm:"column:total_requests"`
		TotalSuccesses int `gorm:"column:total_successes"`
	}

	err := m.db.Model(&EndpointStatistics{}).
		Select("COUNT(*) as total_endpoints, SUM(total_requests) as total_requests, SUM(success_requests) as total_successes").
		Scan(&result).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get statistics summary: %v", err)
	}

	summary := map[string]interface{}{
		"total_endpoints": result.TotalEndpoints,
		"total_requests":  result.TotalRequests,
		"total_successes": result.TotalSuccesses,
		"overall_success_rate": func() float64 {
			if result.TotalRequests == 0 {
				return 0.0
			}
			return float64(result.TotalSuccesses) / float64(result.TotalRequests) * 100.0
		}(),
	}

	return summary, nil
}

// Close closes the database connection
func (m *Manager) Close() error {
	if m.db != nil {
		sqlDB, err := m.db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// GetDatabasePath returns the path to the statistics database file
func (m *Manager) GetDatabasePath() string {
	return m.dbPath
}