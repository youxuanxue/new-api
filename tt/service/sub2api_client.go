//go:build tt
// +build tt

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
)

type Sub2APIConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	APIKey       string
	Timeout      time.Duration
}

var DefaultSub2APIConfig = Sub2APIConfig{
	BaseURL: "https://sub2api.example.com",
	Timeout: 30 * time.Second,
}

var currentSub2APIConfig = DefaultSub2APIConfig
var sub2APIConfigMutex sync.RWMutex

func SetSub2APIConfig(config Sub2APIConfig) {
	sub2APIConfigMutex.Lock()
	defer sub2APIConfigMutex.Unlock()
	if config.Timeout == 0 {
		config.Timeout = DefaultSub2APIConfig.Timeout
	}
	currentSub2APIConfig = config
}

func GetSub2APIConfig() Sub2APIConfig {
	sub2APIConfigMutex.RLock()
	defer sub2APIConfigMutex.RUnlock()
	return currentSub2APIConfig
}

type Sub2APIClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewSub2APIClient(baseURL, apiKey string) *Sub2APIClient {
	timeout := GetSub2APIConfig().Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Sub2APIClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: timeout},
	}
}

// doWithRetry executes an HTTP request with exponential backoff retry for
// transient failures (network errors or 5xx responses).
func (c *Sub2APIClient) doWithRetry(req *http.Request) (*http.Response, error) {
	maxRetries := GetPoolSyncConfig().MaxRetryCount
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			jitterRange := backoff / 2
			if jitterRange <= 0 {
				jitterRange = time.Second
			}
			jitter := time.Duration(rand.Int63n(int64(jitterRange)))
			wait := backoff + jitter
			select {
			case <-time.After(wait):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}

		// Reset body before retry to avoid sending an empty payload on subsequent attempts.
		if attempt > 0 && req.Body != nil {
			if req.GetBody == nil {
				return nil, fmt.Errorf("sub2api request body is not replayable for retry: method=%s", req.Method)
			}
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("sub2api failed to reset request body: %w", err)
			}
			req.Body = body
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			logger.LogError(nil, fmt.Sprintf("[Sub2API] request failed (attempt %d/%d): %v", attempt+1, maxRetries, err))
			continue
		}

		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("sub2api 5xx: status=%d body=%s", resp.StatusCode, string(body))
			logger.LogError(nil, fmt.Sprintf("[Sub2API] server error (attempt %d/%d): %v", attempt+1, maxRetries, lastErr))
			continue
		}

		return resp, nil
	}
	return nil, fmt.Errorf("sub2api request failed after %d attempts: %w", maxRetries, lastErr)
}

// BaseURL returns the configured Sub2API base URL (trimmed, no trailing slash).
func (c *Sub2APIClient) BaseURL() string { return c.baseURL }

// APIKey returns the configured API key.
func (c *Sub2APIClient) APIKey() string { return c.apiKey }

type AccountInfo struct {
	Email        string    `json:"email"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Status       string    `json:"status"`
	QuotaUsed    int64     `json:"quota_used"`
	QuotaTotal   int64     `json:"quota_total"`
	Model        string    `json:"model"`
	LastUsed     time.Time `json:"last_used"`
}

type SyncResult struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Removed int `json:"removed"`
	Errors  int `json:"errors"`
}

type RefreshResult struct {
	Success      bool      `json:"success"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Error        string    `json:"error,omitempty"`
}

type BanStatus struct {
	Email      string    `json:"email"`
	IsBanned   bool      `json:"is_banned"`
	BanReason  string    `json:"ban_reason,omitempty"`
	DetectedAt time.Time `json:"detected_at,omitempty"`
}

func (c *Sub2APIClient) ListAccounts(ctx context.Context) ([]AccountInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/accounts", c.baseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.doWithRetry(req)
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

func (c *Sub2APIClient) GetAccount(ctx context.Context, email string) (*AccountInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/accounts/%s", c.baseURL, email), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.doWithRetry(req)
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

func (c *Sub2APIClient) RefreshToken(ctx context.Context, email string) (*RefreshResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/accounts/%s/refresh", c.baseURL, email), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.doWithRetry(req)
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

func (c *Sub2APIClient) CheckBanStatus(ctx context.Context, email string) (*BanStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/accounts/%s/status", c.baseURL, email), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.doWithRetry(req)
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

func (c *Sub2APIClient) BatchCheckBan(ctx context.Context, emails []string) ([]BanStatus, error) {
	bodyBytes, err := common.Marshal(map[string]any{"emails": emails})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/accounts/batch-status", c.baseURL), strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.doWithRetry(req)
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

func (c *Sub2APIClient) SyncAccounts(ctx context.Context) (*SyncResult, error) {
	remoteAccounts, err := c.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	result := &SyncResult{}
	for _, acc := range remoteAccounts {
		_ = acc
	}
	return result, nil
}

func (c *Sub2APIClient) AddAccount(ctx context.Context, email, password, proxyURL string) error {
	bodyBytes, err := common.Marshal(map[string]any{"email": email, "password": password, "proxy": proxyURL})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/accounts", c.baseURL), strings.NewReader(string(bodyBytes)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("add account failed: status=%d", resp.StatusCode)
	}
	return nil
}

func (c *Sub2APIClient) RemoveAccount(ctx context.Context, email string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/api/v1/accounts/%s", c.baseURL, email), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("remove account failed: status=%d", resp.StatusCode)
	}
	return nil
}

func (c *Sub2APIClient) GetPoolHealth(ctx context.Context) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/pool/health", c.baseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.doWithRetry(req)
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

func StartPoolSyncTask() {
	StartPoolSyncTaskWithContext(context.Background())
}

func StartPoolSyncTaskWithContext(ctx context.Context) {
	logger.LogInfo(nil, "[Sub2API] Starting pool sync task...")
	interval := GetPoolSyncConfig().SyncInterval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	select {
	case <-ctx.Done():
		logger.LogInfo(nil, "[Sub2API] Pool sync task stopped before initial run")
		return
	default:
		runPoolSyncOnce()
	}
	for {
		select {
		case <-ctx.Done():
			logger.LogInfo(nil, "[Sub2API] Pool sync task stopped")
			return
		case <-ticker.C:
			runPoolSyncOnce()
		}
	}
}

func runPoolSyncOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	config := GetSub2APIConfig()
	if config.APIKey == "" {
		logger.LogDebug(nil, "[Sub2API] API key not configured, skipping sync")
		return
	}
	client := NewSub2APIClient(config.BaseURL, config.APIKey)
	result, err := SyncPoolAccounts(ctx, client)
	if err != nil {
		logger.LogError(nil, fmt.Sprintf("[Sub2API] Pool sync failed: %v", err))
		common.SendFeishuAlert(
			"Pool sync failed",
			fmt.Sprintf("sub2api account sync failed: %v", err),
			common.AlertCritical,
		)
		return
	}
	logger.LogInfo(nil, fmt.Sprintf("[Sub2API] Pool sync completed: added=%d, updated=%d, removed=%d, errors=%d", result.Added, result.Updated, result.Removed, result.Errors))
	if result.Errors > 0 {
		common.SendFeishuAlert(
			"Pool sync partial errors",
			fmt.Sprintf("sub2api sync completed with %d item errors", result.Errors),
			common.AlertWarning,
		)
	}
}
