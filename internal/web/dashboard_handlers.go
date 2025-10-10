package web

import (
	"claude-code-codex-companion/internal/endpoint"

	"github.com/gin-gonic/gin"
)

type labelCount struct {
	Label string
	Count int64
}

func (s *AdminServer) handleDashboard(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()

	totalRequests := 0
	successRequests := 0
	activeEndpoints := 0

	type EndpointStats struct {
		*endpoint.Endpoint
		SuccessRate string
	}

	endpointStats := make([]EndpointStats, 0)
	configCounts := map[string]int{"native": 0, "convert": 0, "auto": 0}
	learnedCounts := map[string]int{"native": 0, "converted": 0, "unknown": 0}

	for _, ep := range endpoints {
		totalRequests += ep.TotalRequests
		successRequests += ep.SuccessRequests
		if ep.Status == endpoint.StatusActive {
			activeEndpoints++
		}

		successRate := calculateSuccessRate(ep.SuccessRequests, ep.TotalRequests)

		endpointStats = append(endpointStats, EndpointStats{
			Endpoint:    ep,
			SuccessRate: successRate,
		})

		switch {
		case ep.SupportsResponses == nil:
			configCounts["auto"]++
		case ep.SupportsResponses != nil && *ep.SupportsResponses:
			configCounts["native"]++
		default:
			configCounts["convert"]++
		}

		switch {
		case ep.NativeCodexFormat == nil:
			learnedCounts["unknown"]++
		case *ep.NativeCodexFormat:
			learnedCounts["native"]++
		default:
			learnedCounts["converted"]++
		}
	}

	overallSuccessRate := calculateSuccessRate(successRequests, totalRequests)

	var downgradeStats []labelCount
	var flagStats []labelCount
	var totalDowngrades int64

	if s.logger != nil {
		if logStats, err := s.logger.GetStats(); err == nil {
			if val, ok := logStats["responses_conversion_total"]; ok {
				totalDowngrades = toInt64(val)
			}
			if rows, ok := logStats["responses_conversion_counts"].([]map[string]interface{}); ok {
				for _, row := range rows {
					label := toString(row["endpoint"])
					if label == "" {
						label = "(unknown)"
					}
					downgradeStats = append(downgradeStats, labelCount{Label: label, Count: toInt64(row["count"])})
				}
			}
			if rows, ok := logStats["supports_responses_flag_counts"].([]map[string]interface{}); ok {
				for _, row := range rows {
					label := toString(row["flag"])
					if label == "" {
						label = "unknown"
					}
					flagStats = append(flagStats, labelCount{Label: label, Count: toInt64(row["count"])})
				}
			}
		}
	}

	data := s.mergeTemplateData(c, "dashboard", map[string]interface{}{
		"Title":                 "Claude Proxy Dashboard",
		"TotalEndpoints":        len(endpoints),
		"ActiveEndpoints":       activeEndpoints,
		"TotalRequests":         totalRequests,
		"SuccessRequests":       successRequests,
		"OverallSuccessRate":    overallSuccessRate,
		"Endpoints":             endpointStats,
		"SupportsConfigCounts":  configCounts,
		"SupportsLearnedCounts": learnedCounts,
		"DowngradeStats":        downgradeStats,
		"SupportsFlagCounts":    flagStats,
		"TotalDowngrades":       totalDowngrades,
	})
	s.renderHTML(c, "dashboard.html", data)
}

func (s *AdminServer) handleEndpointsPage(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()

	type EndpointStats struct {
		*endpoint.Endpoint
		SuccessRate string
	}

	endpointStats := make([]EndpointStats, 0)

	for _, ep := range endpoints {
		successRate := calculateSuccessRate(ep.SuccessRequests, ep.TotalRequests)

		endpointStats = append(endpointStats, EndpointStats{
			Endpoint:    ep,
			SuccessRate: successRate,
		})
	}

	data := s.mergeTemplateData(c, "endpoints", map[string]interface{}{
		"Title":     "Endpoints Configuration",
		"Endpoints": endpointStats,
	})
	s.renderHTML(c, "endpoints.html", data)
}

func toInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		return 0
	}
}

func toString(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}
