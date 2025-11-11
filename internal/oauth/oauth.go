package oauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"claude-code-codex-companion/internal/config"
)

// TokenRefreshResponse OAuth token 刷新响应结构
type TokenRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`    // 过期时间（秒）
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope,omitempty"`
}

// TokenRefreshRequest OAuth token 刷新请求结构
type TokenRefreshRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
}

// RefreshToken 刷新 OAuth token
func RefreshToken(oauthConfig *config.OAuthConfig, httpClient *http.Client) (*config.OAuthConfig, error) {
	if oauthConfig == nil {
		return nil, fmt.Errorf("oauth config is nil")
	}

	if oauthConfig.RefreshToken == "" {
		return nil, fmt.Errorf("refresh token is empty")
	}

	log.Printf("[OAuth] Starting token refresh for token_url: %s", oauthConfig.TokenURL)

	// 尝试 JSON 格式请求
	newConfig, err := refreshTokenWithJSON(oauthConfig, httpClient)
	if err != nil {
		log.Printf("[OAuth] JSON format refresh failed: %v, trying form format", err)
		// 如果 JSON 格式失败，尝试 form 格式
		return refreshTokenWithForm(oauthConfig, httpClient)
	}

	return newConfig, nil
}

// refreshTokenWithJSON 使用 JSON 格式刷新 token
func refreshTokenWithJSON(oauthConfig *config.OAuthConfig, httpClient *http.Client) (*config.OAuthConfig, error) {
	// 准备刷新请求
	refreshReq := TokenRefreshRequest{
		GrantType:    "refresh_token",
		RefreshToken: oauthConfig.RefreshToken,
		ClientID:     oauthConfig.ClientID,
	}

	// 序列化请求体
	reqBody, err := json.Marshal(refreshReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal refresh request: %v", err)
	}

	log.Printf("[OAuth] JSON request body: %s", string(reqBody))

	// 创建HTTP请求
	req, err := http.NewRequest("POST", oauthConfig.TokenURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send refresh request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh response: %v", err)
	}

	log.Printf("[OAuth] JSON response status: %d", resp.StatusCode)
	log.Printf("[OAuth] JSON response body: %s", string(respBody))

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return parseTokenResponse(respBody, oauthConfig)
}

// refreshTokenWithForm 使用 form 格式刷新 token
func refreshTokenWithForm(oauthConfig *config.OAuthConfig, httpClient *http.Client) (*config.OAuthConfig, error) {
	// 准备 form 数据
	formData := url.Values{}
	formData.Set("grant_type", "refresh_token")
	formData.Set("refresh_token", oauthConfig.RefreshToken)
	if oauthConfig.ClientID != "" {
		formData.Set("client_id", oauthConfig.ClientID)
	}

	reqBody := formData.Encode()
	log.Printf("[OAuth] Form request body: %s", reqBody)

	// 创建HTTP请求
	req, err := http.NewRequest("POST", oauthConfig.TokenURL, strings.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create form refresh request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send form refresh request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read form refresh response: %v", err)
	}

	log.Printf("[OAuth] Form response status: %d", resp.StatusCode)
	log.Printf("[OAuth] Form response body: %s", string(respBody))

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("form token refresh failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return parseTokenResponse(respBody, oauthConfig)
}

// parseTokenResponse 解析 token 响应
func parseTokenResponse(respBody []byte, oauthConfig *config.OAuthConfig) (*config.OAuthConfig, error) {
	// 解析响应
	var refreshResp TokenRefreshResponse
	if err := json.Unmarshal(respBody, &refreshResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal refresh response: %v", err)
	}

	// 验证响应
	if refreshResp.AccessToken == "" {
		return nil, fmt.Errorf("refresh response missing access_token")
	}

	log.Printf("[OAuth] Token refresh successful, new access_token: %s...", 
		truncateToken(refreshResp.AccessToken))
	if refreshResp.RefreshToken != "" {
		log.Printf("[OAuth] New refresh_token received: %s...", 
			truncateToken(refreshResp.RefreshToken))
	}
	log.Printf("[OAuth] Token expires_in: %d seconds", refreshResp.ExpiresIn)

	// 创建新的 OAuth 配置
	newConfig := *oauthConfig // 复制原配置
	newConfig.AccessToken = refreshResp.AccessToken
	
	// 如果响应中包含新的 refresh token，则更新
	if refreshResp.RefreshToken != "" {
		newConfig.RefreshToken = refreshResp.RefreshToken
	}

	// 计算新的过期时间（当前时间 + expires_in 秒）
	if refreshResp.ExpiresIn > 0 {
		newConfig.ExpiresAt = time.Now().UnixMilli() + (refreshResp.ExpiresIn * 1000)
	} else {
		// 如果没有返回过期时间，设置为1小时后过期
		newConfig.ExpiresAt = time.Now().Add(1 * time.Hour).UnixMilli()
	}

	log.Printf("[OAuth] New token expires_at: %d (%s)", 
		newConfig.ExpiresAt, time.UnixMilli(newConfig.ExpiresAt).Format("2006-01-02 15:04:05"))

	return &newConfig, nil
}

// truncateToken 截断 token 用于日志显示
func truncateToken(token string) string {
	if len(token) > 20 {
		return token[:20]
	}
	return token
}

// IsTokenExpired 检查 token 是否已过期
func IsTokenExpired(oauthConfig *config.OAuthConfig) bool {
	if oauthConfig == nil {
		return true
	}
	
	// 如果过期时间为0或无效（1970年之前），认为需要刷新以获取正确的过期时间
	if oauthConfig.ExpiresAt <= 0 {
		return true
	}
	
	// 提前5分钟刷新，避免在请求过程中过期
	bufferTime := 5 * time.Minute
	expirationTime := time.UnixMilli(oauthConfig.ExpiresAt)
	
	return time.Now().Add(bufferTime).After(expirationTime)
}

// ShouldRefreshToken 检查是否应该刷新token（更宽松的检查）
func ShouldRefreshToken(oauthConfig *config.OAuthConfig) bool {
	if oauthConfig == nil {
		return true
	}
	
	// 如果没有设置过期时间，先尝试使用现有token
	if oauthConfig.ExpiresAt <= 0 {
		return false // 让第一次请求尝试使用现有token
	}
	
	// 提前5分钟刷新
	bufferTime := 5 * time.Minute
	expirationTime := time.UnixMilli(oauthConfig.ExpiresAt)
	
	return time.Now().Add(bufferTime).After(expirationTime)
}

// GetAuthorizationHeader 获取授权头部
func GetAuthorizationHeader(oauthConfig *config.OAuthConfig) string {
	if oauthConfig == nil || oauthConfig.AccessToken == "" {
		return ""
	}
	
	return "Bearer " + oauthConfig.AccessToken
}