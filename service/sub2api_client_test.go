//go:build tt
// +build tt

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// TestSub2APIClientCreation 测试客户端创建
func TestSub2APIClientCreation(t *testing.T) {
	client := NewSub2APIClient("http://localhost:8080", "test-key")
	if client == nil {
		t.Fatal("Expected client to be created")
	}
	if client.baseURL != "http://localhost:8080" {
		t.Errorf("Expected baseURL to be 'http://localhost:8080', got '%s'", client.baseURL)
	}
	if client.apiKey != "test-key" {
		t.Errorf("Expected apiKey to be 'test-key', got '%s'", client.apiKey)
	}
}

// TestSub2APIClientTrimSuffix 测试 URL 尾部斜杠处理
func TestSub2APIClientTrimSuffix(t *testing.T) {
	client := NewSub2APIClient("http://localhost:8080/", "test-key")
	if client.baseURL != "http://localhost:8080" {
		t.Errorf("Expected baseURL to be trimmed, got '%s'", client.baseURL)
	}
}

// TestListAccounts 测试列出账号
func TestListAccounts(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求路径
		if r.URL.Path != "/api/v1/accounts" {
			t.Errorf("Expected path '/api/v1/accounts', got '%s'", r.URL.Path)
		}

		// 验证授权头
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("Expected Authorization 'Bearer test-key', got '%s'", auth)
		}

		// 返回模拟响应
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{
					"email": "test1@example.com",
					"access_token": "token1",
					"refresh_token": "refresh1",
					"status": "available",
					"quota_used": 1000,
					"quota_total": 10000,
					"model": "gpt-4"
				},
				{
					"email": "test2@example.com",
					"access_token": "token2",
					"refresh_token": "refresh2",
					"status": "cooldown",
					"quota_used": 5000,
					"quota_total": 10000,
					"model": "gpt-4"
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewSub2APIClient(server.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	accounts, err := client.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}

	if len(accounts) != 2 {
		t.Errorf("Expected 2 accounts, got %d", len(accounts))
	}

	if accounts[0].Email != "test1@example.com" {
		t.Errorf("Expected first account email 'test1@example.com', got '%s'", accounts[0].Email)
	}
	if accounts[1].Status != "cooldown" {
		t.Errorf("Expected second account status 'cooldown', got '%s'", accounts[1].Status)
	}
}

// TestListAccountsError 测试列出账号错误处理
func TestListAccountsError(t *testing.T) {
	// 创建返回错误的测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewSub2APIClient(server.URL, "test-key")
	ctx := context.Background()

	_, err := client.ListAccounts(ctx)
	if err == nil {
		t.Error("Expected error for 500 response, got nil")
	}
}

// TestRefreshToken 测试刷新 Token
func TestRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求路径
		if r.URL.Path != "/api/v1/accounts/test@example.com/refresh" {
			t.Errorf("Expected path '/api/v1/accounts/test@example.com/refresh', got '%s'", r.URL.Path)
		}

		// 验证请求方法
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"access_token": "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_at": "2025-12-31T23:59:59Z"
		}`))
	}))
	defer server.Close()

	client := NewSub2APIClient(server.URL, "test-key")
	ctx := context.Background()

	result, err := client.RefreshToken(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}
	if result.AccessToken != "new-access-token" {
		t.Errorf("Expected access token 'new-access-token', got '%s'", result.AccessToken)
	}
}

// TestRefreshTokenFailure 测试刷新 Token 失败
func TestRefreshTokenFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request"))
	}))
	defer server.Close()

	client := NewSub2APIClient(server.URL, "test-key")
	ctx := context.Background()

	result, err := client.RefreshToken(ctx, "test@example.com")
	if err == nil {
		t.Error("Expected error for 400 response")
	}
	if result != nil && result.Success {
		t.Error("Expected Success to be false")
	}
}

// TestCheckBanStatus 测试检查封号状态
func TestCheckBanStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/test@example.com/status" {
			t.Errorf("Expected path '/api/v1/accounts/test@example.com/status', got '%s'", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"email": "test@example.com",
			"is_banned": true,
			"ban_reason": "suspicious activity"
		}`))
	}))
	defer server.Close()

	client := NewSub2APIClient(server.URL, "test-key")
	ctx := context.Background()

	status, err := client.CheckBanStatus(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("CheckBanStatus failed: %v", err)
	}

	if !status.IsBanned {
		t.Error("Expected IsBanned to be true")
	}
	if status.BanReason != "suspicious activity" {
		t.Errorf("Expected ban reason 'suspicious activity', got '%s'", status.BanReason)
	}
}

// TestBatchCheckBan 测试批量检查封号状态
func TestBatchCheckBan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/batch-status" {
			t.Errorf("Expected path '/api/v1/accounts/batch-status', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{"email": "test1@example.com", "is_banned": false},
				{"email": "test2@example.com", "is_banned": true, "ban_reason": "rate limit exceeded"}
			]
		}`))
	}))
	defer server.Close()

	client := NewSub2APIClient(server.URL, "test-key")
	ctx := context.Background()

	emails := []string{"test1@example.com", "test2@example.com"}
	statuses, err := client.BatchCheckBan(ctx, emails)
	if err != nil {
		t.Fatalf("BatchCheckBan failed: %v", err)
	}

	if len(statuses) != 2 {
		t.Errorf("Expected 2 statuses, got %d", len(statuses))
	}
	if statuses[0].IsBanned {
		t.Error("Expected first account to not be banned")
	}
	if !statuses[1].IsBanned {
		t.Error("Expected second account to be banned")
	}
}

// TestAddAccount 测试添加账号
func TestAddAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts" {
			t.Errorf("Expected path '/api/v1/accounts', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewSub2APIClient(server.URL, "test-key")
	ctx := context.Background()

	err := client.AddAccount(ctx, "new@example.com", "password123", "http://proxy:8080")
	if err != nil {
		t.Fatalf("AddAccount failed: %v", err)
	}
}

// TestRemoveAccount 测试移除账号
func TestRemoveAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/test@example.com" {
			t.Errorf("Expected path '/api/v1/accounts/test@example.com', got '%s'", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("Expected DELETE method, got %s", r.Method)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewSub2APIClient(server.URL, "test-key")
	ctx := context.Background()

	err := client.RemoveAccount(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("RemoveAccount failed: %v", err)
	}
}

// TestGetPoolHealth 测试获取号池健康状态
func TestGetPoolHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/pool/health" {
			t.Errorf("Expected path '/api/v1/pool/health', got '%s'", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"total": 10,
			"available": 7,
			"cooldown": 2,
			"banned": 1,
			"health_score": 0.85
		}`))
	}))
	defer server.Close()

	client := NewSub2APIClient(server.URL, "test-key")
	ctx := context.Background()

	health, err := client.GetPoolHealth(ctx)
	if err != nil {
		t.Fatalf("GetPoolHealth failed: %v", err)
	}

	if health["total"].(float64) != 10 {
		t.Errorf("Expected total 10, got %v", health["total"])
	}
}

// TestSub2APIConfig 测试配置管理
func TestSub2APIConfig(t *testing.T) {
	config := Sub2APIConfig{
		BaseURL: "https://api.example.com",
		APIKey:  "secret-key",
		Timeout: 60 * time.Second,
	}

	SetSub2APIConfig(config)
	got := GetSub2APIConfig()

	if got.BaseURL != config.BaseURL {
		t.Errorf("Expected BaseURL '%s', got '%s'", config.BaseURL, got.BaseURL)
	}
	if got.APIKey != config.APIKey {
		t.Errorf("Expected APIKey '%s', got '%s'", config.APIKey, got.APIKey)
	}
}

// TestPoolSyncConfig 测试同步配置管理
func TestPoolSyncConfig(t *testing.T) {
	config := PoolSyncConfig{
		SyncInterval:     10 * time.Minute,
		RefreshBefore:    60 * time.Minute,
		BanCheckInterval: 5 * time.Minute,
		MaxRetryCount:    5,
		CooldownDuration: 60 * time.Minute,
	}

	SetPoolSyncConfig(config)
	got := GetPoolSyncConfig()

	if got.SyncInterval != config.SyncInterval {
		t.Errorf("Expected SyncInterval %v, got %v", config.SyncInterval, got.SyncInterval)
	}
	if got.MaxRetryCount != config.MaxRetryCount {
		t.Errorf("Expected MaxRetryCount %d, got %d", config.MaxRetryCount, got.MaxRetryCount)
	}
}

// TestContextCancellation 测试上下文取消
func TestContextCancellation(t *testing.T) {
	// 创建一个会延迟响应的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSub2APIClient(server.URL, "test-key")

	// 创建一个立即取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.ListAccounts(ctx)
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}
}

// TestAccountInfoJSONDecoding 测试账号信息 JSON 解码
func TestAccountInfoJSONDecoding(t *testing.T) {
	jsonData := `{
		"email": "test@example.com",
		"access_token": "token123",
		"refresh_token": "refresh123",
		"expires_at": "2025-12-31T23:59:59Z",
		"status": "available",
		"quota_used": 5000,
		"quota_total": 10000,
		"model": "gpt-4"
	}`

	var account AccountInfo
	err := common.UnmarshalJsonStr(jsonData, &account)
	if err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if account.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", account.Email)
	}
	if account.Status != "available" {
		t.Errorf("Expected status 'available', got '%s'", account.Status)
	}
	if account.QuotaUsed != 5000 {
		t.Errorf("Expected quota_used 5000, got %d", account.QuotaUsed)
	}
}
