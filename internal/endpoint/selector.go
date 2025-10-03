package endpoint

import (
	"fmt"
	"sync"

	"claude-code-codex-companion/internal/utils"
)

type Selector struct {
	endpoints []*Endpoint
	mutex     sync.RWMutex
}

func NewSelector(endpoints []*Endpoint) *Selector {
	return &Selector{
		endpoints: endpoints,
	}
}

func (s *Selector) SelectEndpoint() (*Endpoint, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 转换为 EndpointSorter 接口类型
	sorterEndpoints := make([]utils.EndpointSorter, len(s.endpoints))
	for i, ep := range s.endpoints {
		sorterEndpoints[i] = ep
	}

	// 使用统一的端点选择逻辑
	selected := utils.SelectBestEndpoint(sorterEndpoints)
	if selected == nil {
		return nil, fmt.Errorf("no available endpoints found")
	}

	// 类型断言转换回 *Endpoint
	return selected.(*Endpoint), nil
}

// SelectEndpointWithTags 根据tags选择endpoint
func (s *Selector) SelectEndpointWithTags(tags []string) (*Endpoint, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 转换为 EndpointSorter 接口类型
	sorterEndpoints := make([]utils.EndpointSorter, len(s.endpoints))
	for i, ep := range s.endpoints {
		sorterEndpoints[i] = ep
	}

	// 使用新的标签匹配选择逻辑
	selected := utils.SelectBestEndpointWithTags(sorterEndpoints, tags)
	if selected == nil {
		return nil, fmt.Errorf("no available endpoints match the required tags: %v", tags)
	}

	// 类型断言转换回 *Endpoint
	return selected.(*Endpoint), nil
}

// SelectEndpointWithFormat 根据请求格式选择兼容的端点
// requestFormat: "anthropic" | "openai" | "unknown"
func (s *Selector) SelectEndpointWithFormat(requestFormat string) (*Endpoint, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 根据格式过滤端点
	filteredEndpoints := s.filterEndpointsByFormat(requestFormat)
	if len(filteredEndpoints) == 0 {
		return nil, fmt.Errorf("no available endpoints compatible with format: %s", requestFormat)
	}

	// 转换为 EndpointSorter 接口类型
	sorterEndpoints := make([]utils.EndpointSorter, len(filteredEndpoints))
	for i, ep := range filteredEndpoints {
		sorterEndpoints[i] = ep
	}

	// 使用统一的端点选择逻辑
	selected := utils.SelectBestEndpoint(sorterEndpoints)
	if selected == nil {
		return nil, fmt.Errorf("no available endpoints found for format: %s", requestFormat)
	}

	return selected.(*Endpoint), nil
}

// SelectEndpointWithFormatAndClient 根据请求格式和客户端类型选择兼容的端点
func (s *Selector) SelectEndpointWithFormatAndClient(requestFormat string, clientType string) (*Endpoint, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 根据格式和客户端过滤端点
	filteredEndpoints := s.filterEndpointsByFormatAndClient(requestFormat, clientType)
	if len(filteredEndpoints) == 0 {
		return nil, fmt.Errorf("no available endpoints compatible with format: %s and client: %s", requestFormat, clientType)
	}

	// 转换为 EndpointSorter 接口类型
	sorterEndpoints := make([]utils.EndpointSorter, len(filteredEndpoints))
	for i, ep := range filteredEndpoints {
		sorterEndpoints[i] = ep
	}

	// 使用统一的端点选择逻辑
	selected := utils.SelectBestEndpoint(sorterEndpoints)
	if selected == nil {
		return nil, fmt.Errorf("no available endpoints found for format: %s and client: %s", requestFormat, clientType)
	}

	return selected.(*Endpoint), nil
}

// SelectEndpointWithTagsAndFormat 根据tags和格式选择端点
func (s *Selector) SelectEndpointWithTagsAndFormat(tags []string, requestFormat string) (*Endpoint, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 根据格式过滤端点
	filteredEndpoints := s.filterEndpointsByFormat(requestFormat)
	if len(filteredEndpoints) == 0 {
		return nil, fmt.Errorf("no available endpoints compatible with format: %s", requestFormat)
	}

	// 转换为 EndpointSorter 接口类型
	sorterEndpoints := make([]utils.EndpointSorter, len(filteredEndpoints))
	for i, ep := range filteredEndpoints {
		sorterEndpoints[i] = ep
	}

	// 使用标签匹配选择逻辑
	selected := utils.SelectBestEndpointWithTags(sorterEndpoints, tags)
	if selected == nil {
		return nil, fmt.Errorf("no available endpoints match tags %v and format: %s", tags, requestFormat)
	}

	return selected.(*Endpoint), nil
}

// SelectEndpointWithTagsFormatAndClient 根据tags、格式和客户端类型选择端点
func (s *Selector) SelectEndpointWithTagsFormatAndClient(tags []string, requestFormat string, clientType string) (*Endpoint, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 根据格式和客户端过滤端点
	filteredEndpoints := s.filterEndpointsByFormatAndClient(requestFormat, clientType)
	if len(filteredEndpoints) == 0 {
		return nil, fmt.Errorf("no available endpoints compatible with format: %s and client: %s", requestFormat, clientType)
	}

	// 转换为 EndpointSorter 接口类型
	sorterEndpoints := make([]utils.EndpointSorter, len(filteredEndpoints))
	for i, ep := range filteredEndpoints {
		sorterEndpoints[i] = ep
	}

	// 使用标签匹配选择逻辑
	selected := utils.SelectBestEndpointWithTags(sorterEndpoints, tags)
	if selected == nil {
		return nil, fmt.Errorf("no available endpoints match tags %v, format: %s and client: %s", tags, requestFormat, clientType)
	}

	return selected.(*Endpoint), nil
}

// filterEndpointsByFormat 根据请求格式过滤兼容的端点
func (s *Selector) filterEndpointsByFormat(requestFormat string) []*Endpoint {
	if requestFormat == "" || requestFormat == "unknown" {
		// 格式未知时返回所有端点（保持向后兼容）
		return s.endpoints
	}

	filtered := make([]*Endpoint, 0)
	for _, ep := range s.endpoints {
		if s.isEndpointCompatible(ep, requestFormat) {
			filtered = append(filtered, ep)
		}
	}

	return filtered
}

// filterEndpointsByFormatAndClient 根据请求格式和客户端类型过滤兼容的端点
func (s *Selector) filterEndpointsByFormatAndClient(requestFormat string, clientType string) []*Endpoint {
	filtered := make([]*Endpoint, 0)
	for _, ep := range s.endpoints {
		if s.isEndpointCompatibleWithClient(ep, clientType, requestFormat) {
			filtered = append(filtered, ep)
		}
	}

	return filtered
}

// isEndpointCompatible 判断端点是否与请求格式兼容（不检查客户端类型）
func (s *Selector) isEndpointCompatible(ep *Endpoint, requestFormat string) bool {
	if !ep.Enabled {
		return false
	}

	// 格式兼容性规则：
	// 1. OpenAI 请求 → 只能选择 OpenAI 端点（不支持 OpenAI → Anthropic 转换）
	// 2. Anthropic 请求 → 优先 Anthropic 端点，也可以选择 OpenAI 端点（支持 Anthropic → OpenAI 转换）

	if requestFormat == "openai" {
		// OpenAI 请求只能发到 OpenAI 端点
		return ep.EndpointType == "openai"
	}

	if requestFormat == "anthropic" {
		// Anthropic 请求可以发到任何端点
		// - 发到 Anthropic 端点：直接透传
		// - 发到 OpenAI 端点：自动转换
		return true
	}

	// 未知格式，保持向后兼容
	return true
}

// isEndpointCompatibleWithClient 判断端点是否与客户端类型和请求格式兼容
func (s *Selector) isEndpointCompatibleWithClient(ep *Endpoint, clientType string, requestFormat string) bool {
	if !ep.Enabled {
		return false
	}

	// 移除硬性客户端限制，实现自动判断
	// if len(ep.SupportedClients) > 0 {
	// 	supported := false
	// 	for _, sc := range ep.SupportedClients {
	// 		if sc == clientType {
	// 		supported = true
	// 			break
	// 	}
	// 	if !supported {
	// 		return false // 端点不支持当前客户端类型
	// 	}
	// }
	// 如果 SupportedClients 为空，表示支持所有客户端

	// 2. 检查格式兼容性
	return s.isEndpointCompatible(ep, requestFormat)
}

func (s *Selector) GetAllEndpoints() []*Endpoint {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 返回切片引用而不是拷贝，因为端点数据本身是不可变的
	// 调用者不应该修改返回的切片
	return s.endpoints
}

func (s *Selector) UpdateEndpoints(endpoints []*Endpoint) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.endpoints = endpoints
}