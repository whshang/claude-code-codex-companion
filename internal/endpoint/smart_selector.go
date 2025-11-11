package endpoint

import (
	"fmt"
	"sort"
)

// SmartSelector 智能端点选择器（方案A核心）
type SmartSelector struct {
	endpoints []*Endpoint
}

// NewSmartSelector 创建智能选择器
func NewSmartSelector(endpoints []*Endpoint) *SmartSelector {
	return &SmartSelector{
		endpoints: endpoints,
	}
}

// SelectForClient 为特定客户端类型选择最佳端点
// clientType: "claude_code" | "codex" | "openai" | ""
func (s *SmartSelector) SelectForClient(clientType string) (*Endpoint, error) {
	// 1. 过滤：只选择匹配的端点
	candidates := s.filterByClientType(clientType)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no endpoints available for client type: %s", clientType)
	}

	// 2. 过滤：移除被禁用和被拉黑的端点
	candidates = s.filterActiveEndpoints(candidates)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no active endpoints available for client type: %s", clientType)
	}

    // 3. 排序：性能优先策略
    // - native_format=true 优先（避免转换开销）
    // - 然后按 priority 降序（数字越大优先级越高）
    // - 最后按健康状态和响应时间
	sortedCandidates := s.sortByPerformance(candidates)

	// 4. 返回最优端点
	return sortedCandidates[0], nil
}

// SelectWithTagsForClient 根据tags和客户端类型选择端点
func (s *SmartSelector) SelectWithTagsForClient(tags []string, clientType string) (*Endpoint, error) {
	// 1. 先按客户端类型过滤
	candidates := s.filterByClientType(clientType)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no endpoints available for client type: %s", clientType)
	}

	// 2. 按tags过滤
	candidates = s.filterByTags(candidates, tags)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no endpoints available for tags: %v and client type: %s", tags, clientType)
	}

	// 3. 过滤活跃端点
	candidates = s.filterActiveEndpoints(candidates)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no active endpoints available for tags: %v and client type: %s", tags, clientType)
	}

	// 4. 排序并返回
	sortedCandidates := s.sortByPerformance(candidates)
	return sortedCandidates[0], nil
}

// filterByClientType 按客户端类型过滤端点
func (s *SmartSelector) filterByClientType(clientType string) []*Endpoint {
	var filtered []*Endpoint

	for _, ep := range s.endpoints {
		// 如果端点的client_type为空，表示通用端点，支持所有客户端
		if ep.ClientType == "" || ep.ClientType == clientType {
			filtered = append(filtered, ep)
		}
	}

	return filtered
}

// filterByTags 按tags过滤端点
func (s *SmartSelector) filterByTags(endpoints []*Endpoint, tags []string) []*Endpoint {
	if len(tags) == 0 {
		return endpoints
	}

	var filtered []*Endpoint
	for _, ep := range endpoints {
		if s.hasAnyTag(ep, tags) {
			filtered = append(filtered, ep)
		}
	}

	return filtered
}

// hasAnyTag 检查端点是否包含任意一个指定tag
func (s *SmartSelector) hasAnyTag(ep *Endpoint, tags []string) bool {
	for _, tag := range tags {
		for _, epTag := range ep.Tags {
			if epTag == tag {
				return true
			}
		}
	}
	return false
}

// filterActiveEndpoints 过滤活跃端点（未被禁用且未被拉黑）
func (s *SmartSelector) filterActiveEndpoints(endpoints []*Endpoint) []*Endpoint {
	var filtered []*Endpoint

	for _, ep := range endpoints {
		if ep.Enabled && ep.Status != StatusBlacklisted {
			filtered = append(filtered, ep)
		}
	}

	return filtered
}

// sortByPerformance 按性能排序端点（方案A核心算法）
func (s *SmartSelector) sortByPerformance(endpoints []*Endpoint) []*Endpoint {
	sorted := make([]*Endpoint, len(endpoints))
	copy(sorted, endpoints)

	sort.Slice(sorted, func(i, j int) bool {
		epI, epJ := sorted[i], sorted[j]

		// 第一优先级：native_format=true 优先（性能最优）
		if epI.NativeFormat != epJ.NativeFormat {
			return epI.NativeFormat // true 排在前面
		}

    // 第二优先级：priority 降序（数字越大优先级越高）
    if epI.Priority != epJ.Priority {
            return epI.Priority > epJ.Priority
    }

		// 第三优先级：健康状态
		statusPriority := map[Status]int{
			StatusActive:      1,
			StatusRecovering:  2,
			StatusDegraded:    3,
			StatusInactive:    4,
			StatusBlacklisted: 5,
		}

		iPrio := statusPriority[epI.Status]
		jPrio := statusPriority[epJ.Status]
		if iPrio != jPrio {
			return iPrio < jPrio
		}

		// 第四优先级：平均响应时间（如果有统计数据）
		if epI.Stats != nil && epJ.Stats != nil &&
			epI.Stats.TotalRequests > 0 && epJ.Stats.TotalRequests > 0 {
			// 暂时跳过响应时间比较，因为statistics结构可能不同
			// TODO: 统一statistics接口
		}

		// 默认保持原序
		return false
	})

	return sorted
}

// UpdateEndpoints 更新端点列表
func (s *SmartSelector) UpdateEndpoints(endpoints []*Endpoint) {
	s.endpoints = endpoints
}

// GetEndpointsByClientType 获取特定客户端类型的所有端点（用于统计和调试）
func (s *SmartSelector) GetEndpointsByClientType(clientType string) []*Endpoint {
	return s.filterByClientType(clientType)
}

// GetPerformanceReport 生成性能报告（用于调试和监控）
func (s *SmartSelector) GetPerformanceReport(clientType string) map[string]interface{} {
	candidates := s.filterByClientType(clientType)
	sorted := s.sortByPerformance(candidates)

	report := map[string]interface{}{
		"client_type":    clientType,
		"total_count":    len(s.endpoints),
		"matched_count":  len(candidates),
		"active_count":   len(s.filterActiveEndpoints(candidates)),
		"endpoints":      make([]map[string]interface{}, 0),
	}

	for idx, ep := range sorted {
		epInfo := map[string]interface{}{
			"rank":          idx + 1,
			"name":          ep.Name,
			"native_format": ep.NativeFormat,
			"priority":      ep.Priority,
			"status":        ep.Status,
			"enabled":       ep.Enabled,
			"client_type":   ep.ClientType,
		}

		if ep.Stats != nil && ep.Stats.TotalRequests > 0 {
			successRate := float64(ep.Stats.SuccessRequests) / float64(ep.Stats.TotalRequests) * 100
			epInfo["success_rate"] = fmt.Sprintf("%.2f%%", successRate)
			epInfo["total_requests"] = ep.Stats.TotalRequests
		}

		report["endpoints"] = append(report["endpoints"].([]map[string]interface{}), epInfo)
	}

	return report
}
