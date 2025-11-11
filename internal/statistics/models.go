package statistics

import (
	"time"
)

// EndpointStatistics represents the persistent statistics data for an endpoint
// This corresponds to the endpoint_statistics table in statistics.db
type EndpointStatistics struct {
	// Primary key - stable ID based on endpoint name hash
	ID string `gorm:"primaryKey;column:id;size:64;not null"`
	
	// Endpoint metadata - for display and tracking configuration changes
	Name         string `gorm:"column:name;size:100;not null;index:idx_name"`
	URL          string `gorm:"column:url;size:500;not null"`
	EndpointType string `gorm:"column:endpoint_type;size:50;not null"`
	AuthType     string `gorm:"column:auth_type;size:50;not null"`
	
	// Core statistics data - accumulated over time
	TotalRequests       int `gorm:"column:total_requests;default:0;not null"`
	SuccessRequests     int `gorm:"column:success_requests;default:0;not null"`
	FailureCount        int `gorm:"column:failure_count;default:0;not null"`          // Consecutive failure count
	SuccessiveSuccesses int `gorm:"column:successive_successes;default:0;not null"`  // Consecutive success count
	
	// Timing data
	LastFailure time.Time `gorm:"column:last_failure"`
	
	// Timestamps
	LastUpdated time.Time `gorm:"column:last_updated;default:CURRENT_TIMESTAMP"`
	CreatedAt   time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

// TableName specifies the table name for GORM
func (EndpointStatistics) TableName() string {
	return "endpoint_statistics"
}

// ToMap converts the statistics to a map for JSON serialization
func (e *EndpointStatistics) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"id":                   e.ID,
		"name":                 e.Name,
		"url":                  e.URL,
		"endpoint_type":        e.EndpointType,
		"auth_type":            e.AuthType,
		"total_requests":       e.TotalRequests,
		"success_requests":     e.SuccessRequests,
		"failure_count":        e.FailureCount,
		"successive_successes": e.SuccessiveSuccesses,
		"last_failure":         e.LastFailure,
		"last_updated":         e.LastUpdated,
		"created_at":           e.CreatedAt,
		"updated_at":           e.UpdatedAt,
	}
}

// GetSuccessRate calculates the success rate percentage
func (e *EndpointStatistics) GetSuccessRate() float64 {
	if e.TotalRequests == 0 {
		return 0.0
	}
	return float64(e.SuccessRequests) / float64(e.TotalRequests) * 100.0
}

// IsHealthy returns true if the endpoint appears to be healthy based on statistics
func (e *EndpointStatistics) IsHealthy() bool {
	// Consider healthy if:
	// 1. Has no recent consecutive failures, OR
	// 2. Has recent consecutive successes
	return e.FailureCount == 0 || e.SuccessiveSuccesses > 0
}