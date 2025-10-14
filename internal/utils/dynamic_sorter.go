package utils

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"
)

// DynamicEndpointSorter 动态端点排序器
type DynamicEndpointSorter struct {
	mu            sync.RWMutex
	endpoints     []DynamicEndpoint
	enabled       bool
	updateTicker  *time.Ticker
	updateChannel chan bool
	ctx           context.Context
	cancelFunc    context.CancelFunc
}

// DynamicEndpoint 动态端点接口
type DynamicEndpoint interface {
	EndpointSorter
	GetName() string
	GetLastResponseTime() time.Duration
	GetSuccessRate() float64
	GetFailureCount() int
	GetTotalRequests() int
	SetPriority(int)
}

// NewDynamicEndpointSorter 创建新的动态排序器
func NewDynamicEndpointSorter() *DynamicEndpointSorter {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &DynamicEndpointSorter{
		endpoints:     make([]DynamicEndpoint, 0),
		enabled:       false,
		updateChannel: make(chan bool, 1),
		ctx:           ctx,
		cancelFunc:    cancelFunc,
	}
}

// Enable 启用动态排序
func (des *DynamicEndpointSorter) Enable() {
	des.mu.Lock()
	defer des.mu.Unlock()
	if !des.enabled {
		des.enabled = true
		log.Println("✅ 启用动态端点排序功能")
	}
}

// Disable 禁用动态排序
func (des *DynamicEndpointSorter) Disable() {
	des.mu.Lock()
	defer des.mu.Unlock()
	if des.enabled {
		des.enabled = false
		if des.updateTicker != nil {
			des.updateTicker.Stop()
		}
		close(des.updateChannel)
		des.cancelFunc()
		log.Println("❌ 禁用动态端点排序功能")
	}
}

// SetEndpoints 设置端点列表
func (des *DynamicEndpointSorter) SetEndpoints(endpoints []DynamicEndpoint) {
	des.mu.Lock()
	defer des.mu.Unlock()
	des.endpoints = endpoints
	if des.enabled {
		des.triggerUpdate()
	}
}

// SortAndApply 动态排序并应用到配置
func (des *DynamicEndpointSorter) SortAndApply() {
	des.mu.RLock()
	if !des.enabled || len(des.endpoints) == 0 {
		des.mu.RUnlock()
		return
	}
	des.mu.RUnlock()

	// 分离端点：启用的 vs 禁用的
	var enabledEndpoints []DynamicEndpoint
	var disabledEndpoints []DynamicEndpoint

	for _, ep := range des.endpoints {
		if ep.IsEnabled() {
			enabledEndpoints = append(enabledEndpoints, ep)
		} else {
			disabledEndpoints = append(disabledEndpoints, ep)
		}
	}

	// 只对启用的端点进行性能排序
	if len(enabledEndpoints) > 0 {
		// 按性能指标排序
		sort.Slice(enabledEndpoints, func(i, j int) bool {
			epI := enabledEndpoints[i]
			epJ := enabledEndpoints[j]

			// 1. 状态优先级：可用 > 不可用
			availableI := epI.IsAvailable()
			availableJ := epJ.IsAvailable()
			if availableI != availableJ {
				return availableI // 可用的排在前面
			}

			// 如果状态相同，比较成功率（即使都是可用状态，成功率低的也应该排后面）
			successRateI := epI.GetSuccessRate()
			successRateJ := epJ.GetSuccessRate()
			if successRateI != successRateJ {
				return successRateI > successRateJ
			}

			// 如果都可用且成功率相同，比较响应时间
			if availableI && availableJ {
				// 2. 响应时间：响应快的优先（仅对可用端点）
				responseTimeI := epI.GetLastResponseTime()
				responseTimeJ := epJ.GetLastResponseTime()
				if responseTimeI != responseTimeJ {
					return responseTimeI < responseTimeJ
				}
			}

			// 3. 总请求数：更稳定的优先
			totalI := epI.GetTotalRequests()
			totalJ := epJ.GetTotalRequests()
			if totalI != totalJ {
				return totalI > totalJ
			}

			// 4. 原始优先级：保持原有顺序
			return epI.GetPriority() < epJ.GetPriority()
		})

		// 重新分配启用端点的优先级
		des.mu.Lock()
		defer des.mu.Unlock()
		for i, ep := range enabledEndpoints {
			newPriority := i + 1 // 从1开始编号
			if ep.GetPriority() != newPriority {
				log.Printf("🔄 端点 %s 优先级从 %d 调整为 %d", ep.GetName(), ep.GetPriority(), newPriority)
				ep.SetPriority(newPriority)
			}
		}
	}

	// 禁用的端点保持原有优先级，但确保它们在最后
	des.mu.Lock()
	defer des.mu.Unlock()
	startPriority := len(enabledEndpoints) + 1
	for _, ep := range disabledEndpoints {
		if ep.GetPriority() < startPriority {
			log.Printf("🔒 禁用端点 %s 保持禁用状态，优先级调整为 %d", ep.GetName(), startPriority)
			ep.SetPriority(startPriority)
		}
		startPriority++
	}
}

// sortLoop 排序循环
func (des *DynamicEndpointSorter) sortLoop() {
	for {
		select {
		case <-des.updateChannel:
			des.SortAndApply()
		case <-des.ctx.Done():
			return
		}
	}
}

// triggerUpdate 触发更新
func (des *DynamicEndpointSorter) triggerUpdate() {
	select {
	case des.updateChannel <- true:
	default:
		// 通道已满，跳过这次更新
	}
}

// ForceUpdate 强制立即更新
func (des *DynamicEndpointSorter) ForceUpdate() {
	des.triggerUpdate()
}