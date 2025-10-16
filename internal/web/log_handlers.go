package web

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/logger"
	"claude-code-codex-companion/internal/security"
	"claude-code-codex-companion/internal/i18n"
	"github.com/gin-gonic/gin"
)

func (s *AdminServer) handleLogsPage(c *gin.Context) {
	// 获取参数
	pageStr := c.DefaultQuery("page", strconv.Itoa(config.Default.Pagination.DefaultPage))
	failedOnlyStr := c.DefaultQuery("failed_only", "false")
	
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < config.Default.Pagination.DefaultPage {
		page = config.Default.Pagination.DefaultPage
	}
	
	failedOnly, _ := strconv.ParseBool(failedOnlyStr)
	
	// 每页记录数使用统一默认值
	limit := config.Default.Pagination.DefaultLimit
	offset := (page - 1) * limit
	
	logs, total, _ := s.logger.GetLogs(limit, offset, failedOnly)
	
	// 计算分页信息
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = config.Default.Pagination.MaxPages
	}
	
	// 生成分页数组
	var pages []int
	startPage := page - 5
	if startPage < config.Default.Pagination.DefaultPage {
		startPage = config.Default.Pagination.DefaultPage
	}
	endPage := startPage + 9
	if endPage > totalPages {
		endPage = totalPages
		startPage = endPage - 9
		if startPage < config.Default.Pagination.DefaultPage {
			startPage = config.Default.Pagination.DefaultPage
		}
	}
	
	for i := startPage; i <= endPage; i++ {
		pages = append(pages, i)
	}
	
	data := s.mergeTemplateData(c, "logs", map[string]interface{}{
		"Title":       "Request Logs",
		"Logs":        logs,
		"Total":       total,
		"FailedOnly":  failedOnly,
		"Page":        page,
		"TotalPages":  totalPages,
		"Pages":       pages,
		"HasPrev":     page > 1,
		"HasNext":     page < totalPages,
		"PrevPage":    page - 1,
		"NextPage":    page + 1,
		"Limit":       limit,
	})
	s.renderHTML(c, "logs.html", data)
}

func (s *AdminServer) handleGetLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	failedOnlyStr := c.DefaultQuery("failed_only", "false")
	requestIDStr := c.DefaultQuery("request_id", "")

	// 参数验证
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 || limit > 1000 {
		limit = 100 // 默认值
	}
	
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}
	
	failedOnly, err := strconv.ParseBool(failedOnlyStr)
	if err != nil {
		failedOnly = false
	}

	if requestIDStr != "" {
		// 如果指定了request_id，返回该请求的所有尝试记录
		allLogs, err := s.logger.GetAllLogsByRequestID(requestIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve logs by request ID: " + err.Error(),
				"request_id": requestIDStr,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"logs":  allLogs,
			"total": len(allLogs),
			"request_id": requestIDStr,
		})
		return
	}

	// 增加重试机制获取日志
	var logs []*logger.RequestLog
	var total int
	maxRetries := 3
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		logs, total, err = s.logger.GetLogs(limit, offset, failedOnly)
		if err == nil {
			break
		}
		
		if attempt < maxRetries-1 {
			time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond)
		}
	}
	
	if err != nil {
		fmt.Printf("Failed to retrieve logs after %d attempts: %v\n", maxRetries, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve logs: " + err.Error(),
			"limit": limit,
			"offset": offset,
			"failed_only": failedOnly,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": total,
		"limit": limit,
		"offset": offset,
		"failed_only": failedOnly,
	})
}

// handleCleanupLogs 清理日志
func (s *AdminServer) handleCleanupLogs(c *gin.Context) {
	var request struct {
		Days *int `json:"days" binding:"required,gte=0"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	days := *request.Days

	// 添加安全验证
	if err := security.ValidateLogDays(days); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "log_days_validation_failed", "日志保留天数验证失败: ") + err.Error()})
		return
	}

	// 验证days参数 - 支持0表示清除全部，1, 7, 30表示清除指定天数之前的
	if days < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "days must be >= 0 (0 means delete all logs)"})
		return
	}

	// 执行清理，增加重试机制
	maxRetries := 3
	var deletedCount int64
	var lastErr error
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		deletedCount, err = s.logger.CleanupLogsByDays(days)
		if err == nil {
			break // 成功清理
		}
		
		lastErr = err
		if attempt < maxRetries-1 {
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
			fmt.Printf("Log cleanup retry %d/%d: %v\n", attempt+1, maxRetries, err)
		}
	}

	if lastErr != nil {
		fmt.Printf("Failed to cleanup logs after %d attempts: %v\n", maxRetries, lastErr)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to cleanup logs: " + lastErr.Error(),
			"days": days,
			"attempts": maxRetries,
		})
		return
	}

	message := fmt.Sprintf("Successfully cleaned up %d log entries", deletedCount)
	if days == 0 {
		message = fmt.Sprintf("Successfully deleted all %d log entries", deletedCount)
	} else {
		message = fmt.Sprintf("Successfully deleted %d log entries older than %d days", deletedCount, days)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       message,
		"deleted_count": deletedCount,
		"days": days,
		"timestamp": time.Now().Unix(),
	})
}

// handleGetLogStats 获取日志统计信息
func (s *AdminServer) handleGetLogStats(c *gin.Context) {
	// SQLite存储提供基本统计信息
	stats := map[string]interface{}{
		"storage_type": "sqlite",
		"message": "SQLite storage active with automatic cleanup (30 days retention)",
		"features": []string{
			"Automatic cleanup of logs older than 30 days",
			"Indexed queries for better performance", 
			"Memory efficient storage",
			"ACID transactions",
		},
	}
	
	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// handleExportDebugInfo 导出指定请求的调试信息为ZIP文件
func (s *AdminServer) handleExportDebugInfo(c *gin.Context) {
	requestID := c.Param("request_id")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request ID is required"})
		return
	}

	// 获取请求的所有日志记录
	logs, err := s.logger.GetAllLogsByRequestID(requestID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get logs: " + err.Error()})
		return
	}

	if len(logs) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No logs found for the given request ID"})
		return
	}

	// 生成ZIP文件
	zipData, err := s.generateDebugInfoZip(requestID, logs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate debug info: " + err.Error()})
		return
	}

	// 生成文件名（确保只包含ASCII字符）
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("debug_%s_%s.zip", sanitizeForFilename(requestID), timestamp)

	// 设置响应头
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Length", strconv.Itoa(len(zipData)))

	// 发送ZIP数据
	c.Data(http.StatusOK, "application/zip", zipData)
}

// generateDebugInfoZip 生成包含调试信息的ZIP文件
func (s *AdminServer) generateDebugInfoZip(requestID string, logs []*logger.RequestLog) ([]byte, error) {
	var buf strings.Builder

	// 创建ZIP writer
	zipWriter := zip.NewWriter(&buf)
	defer zipWriter.Close()

	// 生成README.txt
	readmeContent := s.generateReadmeContent(requestID, logs)
	if err := s.addFileToZip(zipWriter, "README.txt", []byte(readmeContent)); err != nil {
		return nil, err
	}

	// 生成meta.json
	metaContent, err := s.generateMetaContent(logs)
	if err != nil {
		return nil, err
	}
	if err := s.addFileToZip(zipWriter, "meta.json", metaContent); err != nil {
		return nil, err
	}

	// 为每次尝试创建目录和文件
	for i, log := range logs {
		attemptDir := fmt.Sprintf("attempts/attempt_%d/", i+1)
		if err := s.addLogFilesToZip(zipWriter, attemptDir, log); err != nil {
			return nil, err
		}
	}

	// 添加相关的端点配置
	if err := s.addEndpointConfigsToZip(zipWriter, logs); err != nil {
		return nil, err
	}

	// 添加相关的tagger配置
	if err := s.addTaggerConfigsToZip(zipWriter, logs); err != nil {
		return nil, err
	}

	zipWriter.Close()
	return []byte(buf.String()), nil
}

// addFileToZip 向ZIP文件中添加文件
func (s *AdminServer) addFileToZip(zipWriter *zip.Writer, filename string, data []byte) error {
	file, err := zipWriter.Create(filename)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	return err
}

// generateReadmeContent 生成README内容
func (s *AdminServer) generateReadmeContent(requestID string, logs []*logger.RequestLog) string {
	var readme strings.Builder
	
	readme.WriteString("DEBUG INFO EXPORT\n")
	readme.WriteString("=================\n\n")
	readme.WriteString(fmt.Sprintf("Request ID: %s\n", requestID))
	readme.WriteString(fmt.Sprintf("Export Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	readme.WriteString(fmt.Sprintf("Total Attempts: %d\n\n", len(logs)))
	
	readme.WriteString("DIRECTORY STRUCTURE:\n")
	readme.WriteString("├── README.txt (this file)\n")
	readme.WriteString("├── meta.json (global metadata for all attempts)\n")
	readme.WriteString("├── attempts/\n")
	for i := range logs {
		readme.WriteString(fmt.Sprintf("│   ├── attempt_%d/\n", i+1))
		readme.WriteString("│   │   ├── meta.json (attempt-specific metadata)\n")
		readme.WriteString("│   │   ├── original_request_headers.txt\n")
		readme.WriteString("│   │   ├── original_request_body.txt\n")
		readme.WriteString("│   │   ├── final_request_headers.txt\n")
		readme.WriteString("│   │   ├── final_request_body.txt\n")
		readme.WriteString("│   │   ├── original_response_headers.txt\n")
		readme.WriteString("│   │   ├── original_response_body.txt\n")
		readme.WriteString("│   │   ├── final_response_headers.txt\n")
		readme.WriteString("│   │   └── final_response_body.txt\n")
	}
	readme.WriteString("├── endpoints/ (endpoint configurations)\n")
	readme.WriteString("└── taggers/ (tagger configurations)\n\n")
	
	readme.WriteString("FILE DESCRIPTIONS:\n")
	readme.WriteString("- meta.json: Global information about the entire request\n")
	readme.WriteString("- attempts/attempt_N/meta.json: Detailed metadata for each attempt\n")
	readme.WriteString("- original_*: Data received by the proxy from client\n")
	readme.WriteString("- final_*: Data sent by the proxy to upstream (after transformations)\n\n")
	
	readme.WriteString("NOTE: Authentication values in endpoint configurations have been sanitized.\n")
	
	return readme.String()
}

// generateMetaContent 生成顶层元数据JSON内容（只包含公共信息）
func (s *AdminServer) generateMetaContent(logs []*logger.RequestLog) ([]byte, error) {
	meta := map[string]interface{}{
		"request_id": logs[0].RequestID,
		"export_timestamp": time.Now().Unix(),
		"total_attempts": len(logs),
		"first_request_time": logs[0].Timestamp.Unix(),
		"last_request_time": logs[len(logs)-1].Timestamp.Unix(),
		"total_duration_ms": func() int64 {
			var total int64
			for _, log := range logs {
				total += log.DurationMs
			}
			return total
		}(),
		"final_status_code": logs[len(logs)-1].StatusCode,
		"has_errors": func() bool {
			for _, log := range logs {
				if log.Error != "" || log.StatusCode >= 400 {
					return true
				}
			}
			return false
		}(),
		"unique_endpoints": func() []string {
			endpoints := make(map[string]bool)
			for _, log := range logs {
				if log.Endpoint != "" {
					endpoints[log.Endpoint] = true
				}
			}
			result := make([]string, 0, len(endpoints))
			for endpoint := range endpoints {
				result = append(result, endpoint)
			}
			return result
		}(),
	}

	return json.MarshalIndent(meta, "", "  ")
}

// generateAttemptMeta 生成单个尝试的元数据
func (s *AdminServer) generateAttemptMeta(log *logger.RequestLog) ([]byte, error) {
	meta := map[string]interface{}{
		"attempt_number": log.AttemptNumber,
		"timestamp": log.Timestamp.Unix(),
		"endpoint": log.Endpoint,
		"method": log.Method,
		"path": log.Path,
		"status_code": log.StatusCode,
		"duration_ms": log.DurationMs,
		"model": log.Model,
		"original_model": log.OriginalModel,
		"rewritten_model": log.RewrittenModel,
		"model_rewrite_applied": log.ModelRewriteApplied,
		"thinking_enabled": log.ThinkingEnabled,
		"thinking_budget_tokens": log.ThinkingBudgetTokens,
		"is_streaming": log.IsStreaming,
		"content_type_override": log.ContentTypeOverride,
		"request_body_size": log.RequestBodySize,
		"response_body_size": log.ResponseBodySize,
		"tags": log.Tags,
		"error": log.Error,
	}

	return json.MarshalIndent(meta, "", "  ")
}

// addLogFilesToZip 添加日志相关文件到ZIP
func (s *AdminServer) addLogFilesToZip(zipWriter *zip.Writer, dirPath string, log *logger.RequestLog) error {
	// 添加attempt的元数据
	attemptMeta, err := s.generateAttemptMeta(log)
	if err != nil {
		return err
	}
	if err := s.addFileToZip(zipWriter, dirPath+"meta.json", attemptMeta); err != nil {
		return err
	}

	// 原始请求头
	if err := s.addFileToZip(zipWriter, dirPath+"original_request_headers.txt", 
		[]byte(s.formatHeaders(log.OriginalRequestHeaders))); err != nil {
		return err
	}

	// 原始请求体
	if err := s.addFileToZip(zipWriter, dirPath+"original_request_body.txt", 
		[]byte(log.OriginalRequestBody)); err != nil {
		return err
	}

	// 最终请求头
	if err := s.addFileToZip(zipWriter, dirPath+"final_request_headers.txt", 
		[]byte(s.formatHeaders(log.FinalRequestHeaders))); err != nil {
		return err
	}

	// 最终请求体
	if err := s.addFileToZip(zipWriter, dirPath+"final_request_body.txt", 
		[]byte(log.FinalRequestBody)); err != nil {
		return err
	}

	// 原始响应头
	if err := s.addFileToZip(zipWriter, dirPath+"original_response_headers.txt", 
		[]byte(s.formatHeaders(log.OriginalResponseHeaders))); err != nil {
		return err
	}

	// 原始响应体
	if err := s.addFileToZip(zipWriter, dirPath+"original_response_body.txt", 
		[]byte(log.OriginalResponseBody)); err != nil {
		return err
	}

	// 最终响应头
	if err := s.addFileToZip(zipWriter, dirPath+"final_response_headers.txt", 
		[]byte(s.formatHeaders(log.FinalResponseHeaders))); err != nil {
		return err
	}

	// 最终响应体
	if err := s.addFileToZip(zipWriter, dirPath+"final_response_body.txt", 
		[]byte(log.FinalResponseBody)); err != nil {
		return err
	}

	return nil
}

// formatHeaders 格式化headers为可读文本
func (s *AdminServer) formatHeaders(headers map[string]string) string {
	if len(headers) == 0 {
		return "(no headers)"
	}

	var result strings.Builder
	for key, value := range headers {
		result.WriteString(fmt.Sprintf("%s: %s\n", key, value))
	}
	return result.String()
}

// addEndpointConfigsToZip is a stub. The logic for adding endpoint configs to the debug zip
// has been disabled as it depended on the old dynamic endpoint configuration.
func (s *AdminServer) addEndpointConfigsToZip(zipWriter *zip.Writer, logs []*logger.RequestLog) error {
	readmeContent := "Endpoint configuration export is disabled due to architectural refactoring."
	if err := s.addFileToZip(zipWriter, "endpoints/README.txt", []byte(readmeContent)); err != nil {
		return err
	}
	return nil
}

// addTaggerConfigsToZip 添加tagger配置到ZIP
func (s *AdminServer) addTaggerConfigsToZip(zipWriter *zip.Writer, logs []*logger.RequestLog) error {
	taggerNames := make(map[string]bool)

	// 收集所有涉及的tagger
	for _, log := range logs {
		for _, tag := range log.Tags {
			// 通过tag找到对应的tagger
			if taggers := s.getTaggersByTag(tag); len(taggers) > 0 {
				for _, tagger := range taggers {
					taggerNames[tagger.Name] = true
				}
			}
		}
	}

	// 为每个tagger创建配置文件
	for taggerName := range taggerNames {
		tagger := s.getTaggerConfigByName(taggerName)
		if tagger != nil {
			taggerJSON, err := json.MarshalIndent(tagger, "", "  ")
			if err != nil {
				return err
			}

			filename := fmt.Sprintf("taggers/tagger_%s.json", sanitizeForFilename(taggerName))
			if err := s.addFileToZip(zipWriter, filename, taggerJSON); err != nil {
				return err
			}
		}
	}

	return nil
}

// getEndpointConfigByName is a stub for a deprecated feature.
// It returns nil as the dynamic endpoint list no longer exists.
func (s *AdminServer) getEndpointConfigByName(name string) interface{} {
	return nil
}

// getTaggerConfigByName 根据名称获取tagger配置
func (s *AdminServer) getTaggerConfigByName(name string) *config.TaggerConfig {
	for i, tagger := range s.config.Tagging.Taggers {
		if tagger.Name == name {
			return &s.config.Tagging.Taggers[i]
		}
	}
	return nil
}

// getTaggersByTag 根据tag获取相关的tagger配置
func (s *AdminServer) getTaggersByTag(tag string) []config.TaggerConfig {
	var result []config.TaggerConfig
	for _, tagger := range s.config.Tagging.Taggers {
		if tagger.Tag == tag {
			result = append(result, tagger)
		}
	}
	return result
}

// sanitizeForFilename 清理文件名，只保留ASCII字符
func sanitizeForFilename(name string) string {
	// 只保留字母、数字、下划线和短横线
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := reg.ReplaceAllString(name, "_")
	
	// 限制长度
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}
	
	return sanitized
}