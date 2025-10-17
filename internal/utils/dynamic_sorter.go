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
	mu             sync.RWMutex
	endpoints      []DynamicEndpoint
	enabled        bool
	updateTicker   *time.Ticker
	updateChannel  chan bool
	ctx            context.Context
	cancelFunc     context.CancelFunc
	configFilePath string // 配置文件路径
	persistCallback func() error // 持久化回调函数
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

// NewDynamicEndpointSorterWithConfig 创建新的动态排序器并指定配置文件路径
func NewDynamicEndpointSorterWithConfig(configPath string) *DynamicEndpointSorter {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &DynamicEndpointSorter{
		endpoints:      make([]DynamicEndpoint, 0),
		enabled:        false,
		updateChannel:  make(chan bool, 1),
		ctx:            ctx,
		cancelFunc:     cancelFunc,
		configFilePath: configPath,
	}
}

// Enable 启用动态排序
func (des *DynamicEndpointSorter) Enable() {
	des.mu.Lock()
	defer des.mu.Unlock()
	if !des.enabled {
		des.enabled = true
		// 启动排序循环
		go des.sortLoop()
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
		// 简化排序逻辑：只看 可用性 > 响应速度
		sort.Slice(enabledEndpoints, func(i, j int) bool {
			epI := enabledEndpoints[i]
			epJ := enabledEndpoints[j]

			// 1. 可用性优先：可用 > 不可用
			availableI := epI.IsAvailable()
			availableJ := epJ.IsAvailable()
			if availableI != availableJ {
				return availableI // 可用的排在前面
			}

			// 2. 响应速度：速度快的优先
			responseTimeI := epI.GetLastResponseTime()
			responseTimeJ := epJ.GetLastResponseTime()

			// 如果响应时间为0（无数据），排在有数据的后面
			if responseTimeI == 0 && responseTimeJ != 0 {
				return false
			}
			if responseTimeI != 0 && responseTimeJ == 0 {
				return true
			}
			if responseTimeI != 0 && responseTimeJ != 0 && responseTimeI != responseTimeJ {
				return responseTimeI < responseTimeJ
			}

			// 3. 原始优先级：保持原有顺序
			return epI.GetPriority() < epJ.GetPriority()
		})
	}

	// 锁定并重新分配所有端点的优先级
	des.mu.Lock()
	defer des.mu.Unlock()

	// 重新分配启用端点的优先级
	currentPriority := 1

	// 首先处理可用的端点
	for _, ep := range enabledEndpoints {
		if ep.IsAvailable() {
			if ep.GetPriority() != currentPriority {
				log.Printf("🔄 端点 %s 优先级从 %d 调整为 %d", ep.GetName(), ep.GetPriority(), currentPriority)
				ep.SetPriority(currentPriority)
			}
			currentPriority++
		}
	}

	// 然后处理不可用但启用的端点
	for _, ep := range enabledEndpoints {
		if !ep.IsAvailable() {
			if ep.GetPriority() != currentPriority {
				log.Printf("⚠️ 端点 %s (不可用) 优先级从 %d 调整为 %d", ep.GetName(), ep.GetPriority(), currentPriority)
				ep.SetPriority(currentPriority)
			}
			currentPriority++
		}
	}

	// 禁用的端点排在最后，按名称排序以保持一致性
	startPriority := currentPriority
	sort.Slice(disabledEndpoints, func(i, j int) bool {
		return disabledEndpoints[i].GetName() < disabledEndpoints[j].GetName()
	})
	for _, ep := range disabledEndpoints {
		if ep.GetPriority() != startPriority {
			log.Printf("🔒 禁用端点 %s 优先级从 %d 调整为 %d", ep.GetName(), ep.GetPriority(), startPriority)
			ep.SetPriority(startPriority)
		}
		startPriority++
	}

	// 如果持久化回调存在，触发配置持久化
	if des.persistCallback != nil {
		if err := des.persistCallback(); err != nil {
			log.Printf("❌ 持久化端点优先级失败: %v", err)
		} else {
			log.Printf("💾 端点优先级已持久化到配置文件")
		}
	}
}

// SetPersistCallback 设置持久化回调函数
func (des *DynamicEndpointSorter) SetPersistCallback(callback func() error) {
	des.mu.Lock()
	defer des.mu.Unlock()
	des.persistCallback = callback
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