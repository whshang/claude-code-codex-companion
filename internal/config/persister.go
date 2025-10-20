package config

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// PersistStrategy 持久化策略
type PersistStrategy int

const (
	StrategyImmediate PersistStrategy = iota // 立即写入
	StrategyThrottled                        // 节流写入
	StrategyBatched                          // 批量写入
)

// ConfigPersister 配置持久化管理器
type ConfigPersister struct {
	mu              sync.RWMutex
	config          *Config
	configPath      string

	// 状态管理
	pendingChanges  bool
	lastWrite       time.Time
	writeCount      int64
	throttleCount   int64

	// 配置
	flushInterval   time.Duration
	maxDirtyTime    time.Duration  // 最大脏数据保留时间

	// 控制
	ticker          *time.Ticker
	stopChan        chan struct{}
	flushChan       chan struct{}  // 手动触发通道

	// 回调
	beforeWrite     func(*Config) error
	afterWrite      func(*Config) error
}

// PersisterConfig 持久化器配置
type PersisterConfig struct {
	FlushInterval  time.Duration  // 写入间隔（默认30秒）
	MaxDirtyTime   time.Duration  // 最大脏数据时间（默认5分钟）
	BeforeWrite    func(*Config) error
	AfterWrite     func(*Config) error
}

// NewConfigPersister 创建配置持久化管理器
func NewConfigPersister(config *Config, path string, cfg *PersisterConfig) *ConfigPersister {
	if cfg == nil {
		cfg = &PersisterConfig{
			FlushInterval: 30 * time.Second,
			MaxDirtyTime:  5 * time.Minute,
		}
	}

	cp := &ConfigPersister{
		config:        config,
		configPath:    path,
		flushInterval: cfg.FlushInterval,
		maxDirtyTime:  cfg.MaxDirtyTime,
		stopChan:      make(chan struct{}),
		flushChan:     make(chan struct{}, 1),
		beforeWrite:   cfg.BeforeWrite,
		afterWrite:    cfg.AfterWrite,
		lastWrite:     time.Now(), // 初始化为当前时间
	}

	return cp
}

// Start 启动持久化管理器
func (cp *ConfigPersister) Start() {
	cp.ticker = time.NewTicker(cp.flushInterval)
	go cp.flushLoop()
	log.Printf("✅ ConfigPersister started with flush interval: %v, max dirty time: %v",
		cp.flushInterval, cp.maxDirtyTime)
}

// Stop 停止持久化管理器（优雅关闭）
func (cp *ConfigPersister) Stop() error {
	log.Println("⏸️  Stopping ConfigPersister...")
	close(cp.stopChan)
	if cp.ticker != nil {
		cp.ticker.Stop()
	}

	// 最后强制写入一次
	return cp.FlushNow()
}

// MarkDirty 标记配置已修改（用于自动排序等高频操作）
func (cp *ConfigPersister) MarkDirty() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if !cp.pendingChanges {
		log.Printf("🔄 Configuration marked as dirty (will flush in %v or when max dirty time %v reached)",
			cp.flushInterval, cp.maxDirtyTime)
	}
	cp.pendingChanges = true
}

// FlushNow 立即写入（用于用户手动操作）
func (cp *ConfigPersister) FlushNow() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// 强制写入，不检查 pendingChanges
	return cp.flushForce()
}

// FlushAsync 异步触发写入
func (cp *ConfigPersister) FlushAsync() {
	select {
	case cp.flushChan <- struct{}{}:
	default:
		// 通道已满，跳过
	}
}

// flushLoop 后台刷新循环
func (cp *ConfigPersister) flushLoop() {
	for {
		select {
		case <-cp.ticker.C:
			// 定期检查
			cp.flushIfNeeded()

		case <-cp.flushChan:
			// 手动触发
			cp.flushIfNeeded()

		case <-cp.stopChan:
			return
		}
	}
}

// flushIfNeeded 条件写入（带节流）
func (cp *ConfigPersister) flushIfNeeded() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// 没有待写入的变更
	if !cp.pendingChanges {
		return
	}

	// 检查是否超过最大脏数据时间（强制写入）
	timeSinceLastWrite := time.Since(cp.lastWrite)
	if timeSinceLastWrite >= cp.maxDirtyTime {
		log.Printf("⚠️ 配置脏数据超过最大时间 %v，强制写入", cp.maxDirtyTime)
		cp.flush()
		return
	}

	// 节流检查：距离上次写入不足间隔时间
	if timeSinceLastWrite < cp.flushInterval {
		cp.throttleCount++
		log.Printf("⏱️  Throttling: skipped flush (last write was %v ago, need %v)",
			timeSinceLastWrite.Round(time.Second), cp.flushInterval)
		return
	}

	// 执行写入
	cp.flush()
}

// flush 实际执行写入（调用者需持有锁）
func (cp *ConfigPersister) flush() error {
	if !cp.pendingChanges {
		return nil
	}

	return cp.flushForce()
}

// flushForce 强制写入（调用者需持有锁，不检查 pendingChanges）
func (cp *ConfigPersister) flushForce() error {
	// 写入前回调
	if cp.beforeWrite != nil {
		if err := cp.beforeWrite(cp.config); err != nil {
			return fmt.Errorf("beforeWrite callback failed: %w", err)
		}
	}

	// 执行写入
	startTime := time.Now()
	if err := SaveConfig(cp.config, cp.configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	writeDuration := time.Since(startTime)

	// 更新状态
	cp.pendingChanges = false
	cp.lastWrite = time.Now()
	cp.writeCount++

	log.Printf("💾 配置已写入 (第 %d 次，节流跳过 %d 次，耗时 %v)",
		cp.writeCount, cp.throttleCount, writeDuration.Round(time.Millisecond))

	// 写入后回调
	if cp.afterWrite != nil {
		if err := cp.afterWrite(cp.config); err != nil {
			log.Printf("⚠️ afterWrite callback failed: %v", err)
		}
	}

	return nil
}

// GetStats 获取统计信息
func (cp *ConfigPersister) GetStats() PersisterStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	return PersisterStats{
		WriteCount:     cp.writeCount,
		ThrottleCount:  cp.throttleCount,
		PendingChanges: cp.pendingChanges,
		LastWrite:      cp.lastWrite,
		FlushInterval:  cp.flushInterval,
	}
}

// PersisterStats 持久化统计
type PersisterStats struct {
	WriteCount     int64
	ThrottleCount  int64
	PendingChanges bool
	LastWrite      time.Time
	FlushInterval  time.Duration
}

// String 返回统计信息的字符串表示
func (ps PersisterStats) String() string {
	status := "clean"
	if ps.PendingChanges {
		status = "dirty"
	}
	return fmt.Sprintf("ConfigPersister Stats: writes=%d, throttled=%d, status=%s, last_write=%v, interval=%v",
		ps.WriteCount, ps.ThrottleCount, status, ps.LastWrite.Format("15:04:05"), ps.FlushInterval)
}
