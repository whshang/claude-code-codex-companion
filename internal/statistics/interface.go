package statistics

// StatisticsManager defines the interface for endpoint statistics management
type StatisticsManager interface {
	// LoadStatistics loads statistics for a specific endpoint ID
	LoadStatistics(endpointID string) (*EndpointStatistics, error)
	
	// LoadStatisticsByName loads statistics for an endpoint by name
	LoadStatisticsByName(endpointName string) (*EndpointStatistics, error)
	
	// SaveStatistics saves or updates endpoint statistics
	SaveStatistics(stats *EndpointStatistics) error
	
	// RecordRequest records a request result and updates statistics
	RecordRequest(endpointID string, success bool) error
	
	// InitializeEndpointStatistics creates new statistics record for an endpoint
	InitializeEndpointStatistics(endpointName, url, endpointType, authType string) (*EndpointStatistics, error)
	
	// UpdateEndpointMetadata updates the metadata fields of an endpoint's statistics
	UpdateEndpointMetadata(endpointID, name, url, endpointType, authType string) error
	
	// DeleteStatistics removes statistics record for an endpoint
	DeleteStatistics(endpointID string) error
	
	// GetAllStatistics returns all endpoint statistics
	GetAllStatistics() ([]*EndpointStatistics, error)
	
	// GetStatisticsSummary returns a summary of all statistics
	GetStatisticsSummary() (map[string]interface{}, error)
	
	// Close closes any resources used by the statistics manager
	Close() error
	
	// GetDatabasePath returns information about the storage location
	GetDatabasePath() string
}

// NewStatisticsManager creates the appropriate statistics manager based on availability
// With pure Go SQLite (modernc.org/sqlite), persistent storage should always work
// Memory fallback is only used in extreme cases (e.g., file permission issues)
func NewStatisticsManager(dataDirectory string) (StatisticsManager, error) {
	// Try to create persistent manager first
	persistentManager, err := NewManager(dataDirectory)
	if err != nil {
		// If persistent manager fails (rare with pure Go SQLite), 
		// fall back to memory-only manager as last resort
		memoryManager := NewMemoryManager()
		return memoryManager, nil // Return no error - memory manager always works
	}
	
	return persistentManager, nil
}