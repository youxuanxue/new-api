// Package service 提供TT业务服务
// sub2api_client.go - Sub2API 客户端集成
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
)

// Sub2APIConfig Sub2API 配置
type Sub2APIConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	APIKey       string
	Timeout      time.Duration
}

// DefaultSub2APIConfig 默认配置
var DefaultSub2APIConfig = Sub2APIConfig{
	BaseURL: "https://sub2api.example.com",
	Timeout: 30 * time.Second,
}

// currentSub2APIConfig 当前配置
var currentSub2APIConfig = DefaultSub2APIConfig
var sub2APIConfigMutex sync.RWMutex

// SetSub2APIConfig 设置配置
func SetSub2APIConfig(config Sub2APIConfig) {
	sub2APIConfigMutex.Lock()
	defer sub2APIConfigMutex.Unlock()
	if config.Timeout == 0 {
		config.Timeout = DefaultSub2APIConfig.Timeout
	}
	currentSub2APIConfig = config
}

// GetSub2APIConfig 获取配置
func GetSub2APIConfig() Sub2APIConfig {
	sub2APIConfigMutex.RLock()
	defer sub2APIConfigMutex.RUnlock()
	return currentSub2APIConfig
}

// Sub2APIClient Sub2API 客户端
type Sub2APIClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewSub2APIClient 创建客户端
func NewSub2APIClient(baseURL, apiKey string) *Sub2APIClient {
	return &Sub2APIClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AccountInfo 账号信息
type AccountInfo struct {
	Email       string    `json:"email"`
	AccessToken string    `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresAt   time.Time `json:"expires_at"`
	Status      string    `json:"status"`
	QuotaUsed   int64     `json:"quota_used"`
	QuotaTotal  int64     `json:"quota_total"`
	Model       string    `json:"model"`
	LastUsed    time.Time `json:"last_used"`
}

// SyncResult 同步结果
type SyncResult struct {
	Added    int `json:"added"`
	Updated  int `json:"updated"`
	Removed  int `json:"removed"`
	Errors   int `json:"errors"`
}

// RefreshResult Token 刷新结果
type RefreshResult struct {
	Success      bool      `json:"success"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Error        string    `json:"error,omitempty"`
}

// BanStatus 封号状态
type BanStatus struct {
	Email       string    `json:"email"`
	IsBanned    bool      `json:"is_banned"`
	BanReason   string    `json:"ban_reason,omitempty"`
	DetectedAt  time.Time `json:"detected_at,omitempty"`
}

// ListAccounts 列出账号
func (c *Sub2APIClient) ListAccounts(ctx context.Context) ([]AccountInfo, error) {
	url := fmt.Sprintf("%s/api/v1/accounts", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sub2api list accounts failed: status=%d", resp.StatusCode)
	}

	var result struct {
		Data []AccountInfo `json:"data"`
	}
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetAccount 获取单个账号
func (c *Sub2APIClient) GetAccount(ctx context.Context, email string) (*AccountInfo, error) {
	url := fmt.Sprintf("%s/api/v1/accounts/%s", c.baseURL, email)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("account not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sub2api get account failed: status=%d", resp.StatusCode)
	}

	var account AccountInfo
	if err := common.DecodeJson(resp.Body, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// RefreshToken 刷新 Token
func (c *Sub2APIClient) RefreshToken(ctx context.Context, email string) (*RefreshResult, error) {
	url := fmt.Sprintf("%s/api/v1/accounts/%s/refresh", c.baseURL, email)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result RefreshResult
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		result.Success = false
		result.Error = string(body)
		return &result, fmt.Errorf("refresh failed: status=%d", resp.StatusCode)
	}

	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}

	result.Success = true
	return &result, nil
}

// CheckBanStatus 检查封号状态
func (c *Sub2APIClient) CheckBanStatus(ctx context.Context, email string) (*BanStatus, error) {
	url := fmt.Sprintf("%s/api/v1/accounts/%s/status", c.baseURL, email)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status BanStatus
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check ban status failed: status=%d", resp.StatusCode)
	}

	if err := common.DecodeJson(resp.Body, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// BatchCheckBan 批量检查封号状态
func (c *Sub2APIClient) BatchCheckBan(ctx context.Context, emails []string) ([]BanStatus, error) {
	url := fmt.Sprintf("%s/api/v1/accounts/batch-status", c.baseURL)

	payload := map[string]interface{}{
		"emails": emails,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []BanStatus `json:"data"`
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("batch check ban failed: status=%d", resp.StatusCode)
	}

	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// SyncAccounts 同步账号到本地数据库
func (c *Sub2APIClient) SyncAccounts(ctx context.Context) (*SyncResult, error) {
	// 获取远程账号列表
	remoteAccounts, err := c.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}

	// 这里需要与本地数据库同步
	// 由于需要访问 model 层，我们将在服务层实现具体逻辑
	for _, acc := range remoteAccounts {
		_ = acc // 占位，实际同步逻辑在 pool_sync.go 中
	}

	return result, nil
}

// AddAccount 添加账号到 Sub2API
func (c *Sub2APIClient) AddAccount(ctx context.Context, email, password, proxyURL string) error {
	url := fmt.Sprintf("%s/api/v1/accounts", c.baseURL)

	payload := map[string]interface{}{
		"email":    email,
		"password": password,
		"proxy":    proxyURL,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("add account failed: status=%d", resp.StatusCode)
	}

	return nil
}

// RemoveAccount 从 Sub2API 移除账号
func (c *Sub2APIClient) RemoveAccount(ctx context.Context, email string) error {
	url := fmt.Sprintf("%s/api/v1/accounts/%s", c.baseURL, email)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("remove account failed: status=%d", resp.StatusCode)
	}

	return nil
}

// GetPoolHealth 获取号池健康状态
func (c *Sub2APIClient) GetPoolHealth(ctx context.Context) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/pool/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get pool health failed: status=%d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// init 注册后台任务
func init() {
	// 启动账号池同步任务
	go func() {
		time.Sleep(5 * time.Second) // 等待服务启动
		StartPoolSyncTask()
	}()
}

// StartPoolSyncTask 启动账号池同步任务
func StartPoolSyncTask() {
	logger.LogInfo(nil, "[Sub2API] Starting pool sync task...")

	interval := 5 * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 首次立即执行
	runPoolSyncOnce()

	for range ticker.C {
		runPoolSyncOnce()
	}
}

// runPoolSyncOnce 执行一次同步
func runPoolSyncOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	config := GetSub2APIConfig()
	if config.APIKey == "" {
		logger.LogDebug(nil, "[Sub2API] API key not configured, skipping sync")
		return
	}

	client := NewSub2APIClient(config.BaseURL, config.APIKey)

	// 同步账号
	result, err := SyncPoolAccounts(ctx, client)
	if err != nil {
		logger.LogError(nil, fmt.Sprintf("[Sub2API] Pool sync failed: %v", err))
		return
	}

	logger.LogInfo(nil, fmt.Sprintf("[Sub2API] Pool sync completed: added=%d, updated=%d, removed=%d, errors=%d",
		result.Added, result.Updated, result.Removed, result.Errors))
}
