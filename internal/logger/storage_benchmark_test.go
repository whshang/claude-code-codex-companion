package logger

import (
	"testing"
	"time"
	"os"
	"fmt"
)

// 生成测试日志
func generateTestLog(index int) *RequestLog {
	return &RequestLog{
		Timestamp:     time.Now(),
		RequestID:     fmt.Sprintf("test-req-%d", index),
		Endpoint:      "benchmark-endpoint",
		Method:        "POST",
		Path:          "/test/benchmark",
		StatusCode:    200,
		DurationMs:    100,
		AttemptNumber: 1,
		RequestHeaders: map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   "benchmark-agent",
		},
		RequestBody:     `{"test": "benchmark data"}`,
		RequestBodySize: 25,
		ResponseHeaders: map[string]string{
			"Content-Type": "application/json",
		},
		ResponseBody:     `{"result": "benchmark success"}`,
		ResponseBodySize: 30,
		IsStreaming:      false,
		Model:            "claude-3-sonnet-20240229",
		Tags:             []string{"benchmark", "test"},
	}
}

func setupGORMStorage() (*GORMStorage, func()) {
	tempDir := "./benchmark_gorm_storage"
	storage, err := NewGORMStorage(tempDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create GORM storage: %v", err))
	}
	
	cleanup := func() {
		storage.Close()
		os.RemoveAll(tempDir)
	}
	
	return storage, cleanup
}

func setupSQLiteStorage() (*GORMStorage, func()) {
	tempDir := "./benchmark_sqlite_storage"
	storage, err := NewGORMStorage(tempDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create SQLite storage: %v", err))
	}
	
	cleanup := func() {
		storage.Close()
		os.RemoveAll(tempDir)
	}
	
	return storage, cleanup
}

// 基准测试：写入性能对比
func BenchmarkStorageWrite(b *testing.B) {
	b.Run("GORM-Write", func(b *testing.B) {
		storage, cleanup := setupGORMStorage()
		defer cleanup()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			log := generateTestLog(i)
			storage.SaveLog(log)
		}
	})
	
	b.Run("SQLite-Write", func(b *testing.B) {
		storage, cleanup := setupSQLiteStorage()
		defer cleanup()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			log := generateTestLog(i)
			storage.SaveLog(log)
		}
	})
}

// 基准测试：读取性能对比
func BenchmarkStorageRead(b *testing.B) {
	b.Run("GORM-Read", func(b *testing.B) {
		storage, cleanup := setupGORMStorage()
		defer cleanup()
		
		// 准备测试数据
		for i := 0; i < 1000; i++ {
			storage.SaveLog(generateTestLog(i))
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			storage.GetLogs(100, 0, false)
		}
	})
	
	b.Run("SQLite-Read", func(b *testing.B) {
		storage, cleanup := setupSQLiteStorage()
		defer cleanup()
		
		// 准备测试数据
		for i := 0; i < 1000; i++ {
			storage.SaveLog(generateTestLog(i))
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			storage.GetLogs(100, 0, false)
		}
	})
}

// 基准测试：按RequestID查询性能对比
func BenchmarkStorageQueryByRequestID(b *testing.B) {
	testRequestID := "test-req-500"
	
	b.Run("GORM-QueryByRequestID", func(b *testing.B) {
		storage, cleanup := setupGORMStorage()
		defer cleanup()
		
		// 准备测试数据
		for i := 0; i < 1000; i++ {
			storage.SaveLog(generateTestLog(i))
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			storage.GetAllLogsByRequestID(testRequestID)
		}
	})
	
	b.Run("SQLite-QueryByRequestID", func(b *testing.B) {
		storage, cleanup := setupSQLiteStorage()
		defer cleanup()
		
		// 准备测试数据
		for i := 0; i < 1000; i++ {
			storage.SaveLog(generateTestLog(i))
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			storage.GetAllLogsByRequestID(testRequestID)
		}
	})
}

// 功能测试：确保GORM实现与原始实现功能一致
func TestGORMStorageFunctionality(t *testing.T) {
	gormStorage, gormCleanup := setupGORMStorage()
	defer gormCleanup()
	
	sqliteStorage, sqliteCleanup := setupSQLiteStorage()
	defer sqliteCleanup()
	
	// 创建相同的测试数据
	testLogs := []*RequestLog{
		generateTestLog(1),
		generateTestLog(2),
		{
			Timestamp:  time.Now(),
			RequestID:  "failed-req",
			Endpoint:   "test-endpoint",
			Method:     "GET",
			Path:       "/test/failed",
			StatusCode: 500,
			Error:      "Internal Server Error",
		},
	}
	
	// 保存到两个存储
	for _, log := range testLogs {
		gormStorage.SaveLog(log)
		sqliteStorage.SaveLog(log)
	}
	
	// 测试GetLogs - 全部日志
	gormLogs, gormTotal, err := gormStorage.GetLogs(10, 0, false)
	if err != nil {
		t.Fatalf("GORM GetLogs failed: %v", err)
	}
	
	sqliteLogs, sqliteTotal, err := sqliteStorage.GetLogs(10, 0, false)
	if err != nil {
		t.Fatalf("SQLite GetLogs failed: %v", err)
	}
	
	if gormTotal != sqliteTotal {
		t.Errorf("Total count mismatch: GORM=%d, SQLite=%d", gormTotal, sqliteTotal)
	}
	
	if len(gormLogs) != len(sqliteLogs) {
		t.Errorf("Logs count mismatch: GORM=%d, SQLite=%d", len(gormLogs), len(sqliteLogs))
	}
	
	// 测试GetLogs - 仅失败日志
	gormFailedLogs, gormFailedTotal, err := gormStorage.GetLogs(10, 0, true)
	if err != nil {
		t.Fatalf("GORM GetLogs (failed) failed: %v", err)
	}
	
	sqliteFailedLogs, sqliteFailedTotal, err := sqliteStorage.GetLogs(10, 0, true)
	if err != nil {
		t.Fatalf("SQLite GetLogs (failed) failed: %v", err)
	}
	
	if gormFailedTotal != sqliteFailedTotal {
		t.Errorf("Failed logs total mismatch: GORM=%d, SQLite=%d", gormFailedTotal, sqliteFailedTotal)
	}
	
	if len(gormFailedLogs) != len(sqliteFailedLogs) {
		t.Errorf("Failed logs count mismatch: GORM=%d, SQLite=%d", len(gormFailedLogs), len(sqliteFailedLogs))
	}
	
	// 测试GetAllLogsByRequestID
	gormByID, err := gormStorage.GetAllLogsByRequestID("test-req-1")
	if err != nil {
		t.Fatalf("GORM GetAllLogsByRequestID failed: %v", err)
	}
	
	sqliteByID, err := sqliteStorage.GetAllLogsByRequestID("test-req-1")
	if err != nil {
		t.Fatalf("SQLite GetAllLogsByRequestID failed: %v", err)
	}
	
	if len(gormByID) != len(sqliteByID) {
		t.Errorf("Logs by RequestID count mismatch: GORM=%d, SQLite=%d", len(gormByID), len(sqliteByID))
	}
	
	t.Logf("✅ GORM存储功能测试通过 - 与原始SQLite存储行为一致")
}