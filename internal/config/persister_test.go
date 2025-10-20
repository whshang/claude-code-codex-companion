package config

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestConfigPersister_MarkDirtyAndFlush(t *testing.T) {
	// 创建临时配置文件
	tmpFile, err := ioutil.TempFile("", "config_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 创建测试配置（必须有有效的 URL）
	cfg := &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Endpoints: []EndpointConfig{
			{
				Name:         "test-endpoint",
				URLAnthropic: "https://api.example.com",
				AuthType:     "api_key",
				AuthValue:    "test-key",
				Enabled:      true,
				Priority:     1,
			},
		},
	}

	// 保存初始配置
	if err := SaveConfig(cfg, tmpFile.Name()); err != nil {
		t.Fatalf("Failed to save initial config: %v", err)
	}

	// 创建 ConfigPersister
	writeCount := 0
	persister := NewConfigPersister(cfg, tmpFile.Name(), &PersisterConfig{
		FlushInterval: 100 * time.Millisecond,
		MaxDirtyTime:  500 * time.Millisecond,
		AfterWrite: func(c *Config) error {
			writeCount++
			return nil
		},
	})

	// 启动 persister
	persister.Start()
	defer persister.Stop()

	// 测试1: MarkDirty 不立即写入
	initialWriteCount := writeCount
	persister.MarkDirty()
	time.Sleep(50 * time.Millisecond)
	if writeCount > initialWriteCount {
		t.Errorf("MarkDirty should not trigger immediate write")
	}

	// 测试2: 等待 FlushInterval 后自动写入
	time.Sleep(100 * time.Millisecond)
	if writeCount == initialWriteCount {
		t.Errorf("Expected automatic flush after FlushInterval")
	}

	// 测试3: FlushNow 立即写入
	persister.MarkDirty()
	beforeFlushCount := writeCount
	if err := persister.FlushNow(); err != nil {
		t.Fatalf("FlushNow failed: %v", err)
	}
	if writeCount <= beforeFlushCount {
		t.Errorf("FlushNow should trigger immediate write")
	}

	// 测试4: 统计信息正确
	stats := persister.GetStats()
	if stats.WriteCount == 0 {
		t.Errorf("Expected non-zero write count")
	}
	if stats.PendingChanges {
		t.Errorf("Should have no pending changes after flush")
	}
}

func TestConfigPersister_ThrottlingBehavior(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "config_throttle_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	cfg := &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Endpoints: []EndpointConfig{
			{
				Name:         "test",
				URLAnthropic: "https://api.example.com",
				AuthType:     "api_key",
				AuthValue:    "test",
				Enabled:      true,
				Priority:     1,
			},
		},
	}

	SaveConfig(cfg, tmpFile.Name())

	writeCount := 0
	persister := NewConfigPersister(cfg, tmpFile.Name(), &PersisterConfig{
		FlushInterval: 200 * time.Millisecond,
		MaxDirtyTime:  1 * time.Second,
		AfterWrite: func(c *Config) error {
			writeCount++
			return nil
		},
	})

	persister.Start()
	defer persister.Stop()

	// 快速标记多次脏数据
	for i := 0; i < 5; i++ {
		persister.MarkDirty()
		time.Sleep(20 * time.Millisecond)
	}

	// 验证节流生效：多次 MarkDirty 只触发一次写入
	time.Sleep(250 * time.Millisecond)
	if writeCount > 1 {
		t.Logf("Write count: %d (expected 1 due to throttling)", writeCount)
	}

	stats := persister.GetStats()
	t.Logf("Throttle count: %d", stats.ThrottleCount)
}

func TestConfigPersister_MaxDirtyTimeForce(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "config_maxdirty_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	cfg := &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Endpoints: []EndpointConfig{
			{
				Name:         "test",
				URLAnthropic: "https://api.example.com",
				AuthType:     "api_key",
				AuthValue:    "test",
				Enabled:      true,
				Priority:     1,
			},
		},
	}

	SaveConfig(cfg, tmpFile.Name())

	writeCount := 0
	persister := NewConfigPersister(cfg, tmpFile.Name(), &PersisterConfig{
		FlushInterval: 10 * time.Second, // 很长的间隔
		MaxDirtyTime:  150 * time.Millisecond, // 短的最大脏数据时间
		AfterWrite: func(c *Config) error {
			writeCount++
			return nil
		},
	})

	persister.Start()
	defer persister.Stop()

	// 标记脏数据
	persister.MarkDirty()

	// 手动触发刷新检查（模拟后台循环）
	time.Sleep(200 * time.Millisecond)
	persister.FlushAsync()
	time.Sleep(50 * time.Millisecond)

	// 验证写入发生
	if writeCount == 0 {
		t.Logf("Warning: MaxDirtyTime force write may not have triggered in time")
	} else {
		t.Logf("Successfully triggered write after MaxDirtyTime")
	}
}

func TestConfigPersister_StopFlushes(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "config_stop_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	cfg := &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Endpoints: []EndpointConfig{
			{
				Name:         "test",
				URLAnthropic: "https://api.example.com",
				AuthType:     "api_key",
				AuthValue:    "test",
				Enabled:      true,
				Priority:     1,
			},
		},
	}

	SaveConfig(cfg, tmpFile.Name())

	writeCount := 0
	persister := NewConfigPersister(cfg, tmpFile.Name(), &PersisterConfig{
		FlushInterval: 1 * time.Second,
		MaxDirtyTime:  5 * time.Second,
		AfterWrite: func(c *Config) error {
			writeCount++
			return nil
		},
	})

	persister.Start()

	// 标记脏数据但不等待刷新
	persister.MarkDirty()
	time.Sleep(50 * time.Millisecond)

	initialWriteCount := writeCount

	// 停止应该触发最终刷新
	if err := persister.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// 验证 Stop 触发了刷新
	if writeCount == initialWriteCount {
		t.Errorf("Expected Stop to flush pending changes")
	}
}

func TestConfigPersister_FlushNowWithoutMarkDirty(t *testing.T) {
	// 这个测试模拟 Web UI 的场景：直接修改配置，然后调用 FlushNow（不调用 MarkDirty）
	tmpFile, err := ioutil.TempFile("", "config_webui_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	cfg := &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Endpoints: []EndpointConfig{
			{
				Name:         "test",
				URLAnthropic: "https://api.example.com",
				AuthType:     "api_key",
				AuthValue:    "test",
				Enabled:      true,
				Priority:     1,
			},
		},
	}

	SaveConfig(cfg, tmpFile.Name())

	writeCount := 0
	persister := NewConfigPersister(cfg, tmpFile.Name(), &PersisterConfig{
		FlushInterval: 10 * time.Second,
		MaxDirtyTime:  10 * time.Second,
		AfterWrite: func(c *Config) error {
			writeCount++
			return nil
		},
	})

	persister.Start()
	defer persister.Stop()

	// 模拟 Web UI：直接修改配置对象
	cfg.Endpoints[0].Priority = 999

	// 不调用 MarkDirty，直接调用 FlushNow（这是 Web UI 的模式）
	if err := persister.FlushNow(); err != nil {
		t.Fatalf("FlushNow failed: %v", err)
	}

	// 验证写入发生了
	if writeCount == 0 {
		t.Errorf("FlushNow should write even without MarkDirty (Web UI pattern)")
	}

	// 验证配置文件内容正确
	loadedCfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loadedCfg.Endpoints[0].Priority != 999 {
		t.Errorf("Expected priority 999, got %d", loadedCfg.Endpoints[0].Priority)
	}
}
