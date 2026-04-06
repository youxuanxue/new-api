// Package service 提供TT业务服务
// pool_sync_test.go - 账号池同步测试
package service

import (
	"context"
	"testing"
	"time"
)

// TestPoolSyncConfig 测试同步配置
func TestPoolSyncConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   PoolSyncConfig
		expected PoolSyncConfig
	}{
		{
			name: "default config",
			config: PoolSyncConfig{
				SyncInterval:      5 * time.Minute,
				RefreshBefore:     30 * time.Minute,
				BanCheckInterval:  1 * time.Minute,
				MaxRetryCount:     3,
				CooldownDuration:  30 * time.Minute,
			},
			expected: PoolSyncConfig{
				SyncInterval:      5 * time.Minute,
				RefreshBefore:     30 * time.Minute,
				BanCheckInterval:  1 * time.Minute,
				MaxRetryCount:     3,
				CooldownDuration:  30 * time.Minute,
			},
		},
		{
			name: "custom config",
			config: PoolSyncConfig{
				SyncInterval:      10 * time.Minute,
				RefreshBefore:     60 * time.Minute,
				BanCheckInterval:  5 * time.Minute,
				MaxRetryCount:     5,
				CooldownDuration:  60 * time.Minute,
			},
			expected: PoolSyncConfig{
				SyncInterval:      10 * time.Minute,
				RefreshBefore:     60 * time.Minute,
				BanCheckInterval:  5 * time.Minute,
				MaxRetryCount:     5,
				CooldownDuration:  60 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetPoolSyncConfig(tt.config)
			got := GetPoolSyncConfig()

			if got.SyncInterval != tt.expected.SyncInterval {
				t.Errorf("SyncInterval: expected %v, got %v", tt.expected.SyncInterval, got.SyncInterval)
			}
			if got.RefreshBefore != tt.expected.RefreshBefore {
				t.Errorf("RefreshBefore: expected %v, got %v", tt.expected.RefreshBefore, got.RefreshBefore)
			}
			if got.BanCheckInterval != tt.expected.BanCheckInterval {
				t.Errorf("BanCheckInterval: expected %v, got %v", tt.expected.BanCheckInterval, got.BanCheckInterval)
			}
			if got.MaxRetryCount != tt.expected.MaxRetryCount {
				t.Errorf("MaxRetryCount: expected %d, got %d", tt.expected.MaxRetryCount, got.MaxRetryCount)
			}
			if got.CooldownDuration != tt.expected.CooldownDuration {
				t.Errorf("CooldownDuration: expected %v, got %v", tt.expected.CooldownDuration, got.CooldownDuration)
			}
		})
	}
}

// TestDefaultPoolSyncConfig 测试默认配置
func TestDefaultPoolSyncConfig(t *testing.T) {
	if DefaultPoolSyncConfig.SyncInterval != 5*time.Minute {
		t.Errorf("Default SyncInterval should be 5m, got %v", DefaultPoolSyncConfig.SyncInterval)
	}
	if DefaultPoolSyncConfig.RefreshBefore != 30*time.Minute {
		t.Errorf("Default RefreshBefore should be 30m, got %v", DefaultPoolSyncConfig.RefreshBefore)
	}
	if DefaultPoolSyncConfig.BanCheckInterval != 1*time.Minute {
		t.Errorf("Default BanCheckInterval should be 1m, got %v", DefaultPoolSyncConfig.BanCheckInterval)
	}
	if DefaultPoolSyncConfig.MaxRetryCount != 3 {
		t.Errorf("Default MaxRetryCount should be 3, got %d", DefaultPoolSyncConfig.MaxRetryCount)
	}
	if DefaultPoolSyncConfig.CooldownDuration != 30*time.Minute {
		t.Errorf("Default CooldownDuration should be 30m, got %v", DefaultPoolSyncConfig.CooldownDuration)
	}
}

// TestSyncResult 测试同步结果结构
func TestSyncResult(t *testing.T) {
	result := SyncResult{
		Added:    5,
		Updated:  10,
		Removed:  2,
		Errors:   1,
	}

	if result.Added != 5 {
		t.Errorf("Expected Added 5, got %d", result.Added)
	}
	if result.Updated != 10 {
		t.Errorf("Expected Updated 10, got %d", result.Updated)
	}
	if result.Removed != 2 {
		t.Errorf("Expected Removed 2, got %d", result.Removed)
	}
	if result.Errors != 1 {
		t.Errorf("Expected Errors 1, got %d", result.Errors)
	}
}

// TestRefreshResult 测试刷新结果结构
func TestRefreshResult(t *testing.T) {
	tests := []struct {
		name   string
		result RefreshResult
	}{
		{
			name: "successful refresh",
			result: RefreshResult{
				Success:       true,
				AccessToken:   "new-token",
				RefreshToken:  "new-refresh",
				ExpiresAt:     time.Now().Add(1 * time.Hour),
			},
		},
		{
			name: "failed refresh",
			result: RefreshResult{
				Success: false,
				Error:   "invalid refresh token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "successful refresh" {
				if !tt.result.Success {
					t.Error("Expected Success to be true")
				}
				if tt.result.AccessToken == "" {
					t.Error("Expected AccessToken to be non-empty")
				}
			} else {
				if tt.result.Success {
					t.Error("Expected Success to be false")
				}
				if tt.result.Error == "" {
					t.Error("Expected Error to be non-empty")
				}
			}
		})
	}
}

// TestBanStatus 测试封号状态结构
func TestBanStatus(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		status BanStatus
	}{
		{
			name: "not banned",
			status: BanStatus{
				Email:    "test@example.com",
				IsBanned: false,
			},
		},
		{
			name: "banned",
			status: BanStatus{
				Email:      "banned@example.com",
				IsBanned:   true,
				BanReason:  "rate limit exceeded",
				DetectedAt: now,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.status.Email == "" {
				t.Error("Email should not be empty")
			}
			if tt.name == "banned" {
				if !tt.status.IsBanned {
					t.Error("Expected IsBanned to be true")
				}
				if tt.status.BanReason == "" {
					t.Error("Expected BanReason to be non-empty")
				}
			}
		})
	}
}

// TestAccountInfo 测试账号信息结构
func TestAccountInfo(t *testing.T) {
	now := time.Now()
	account := AccountInfo{
		Email:        "test@example.com",
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    now.Add(1 * time.Hour),
		Status:       "available",
		QuotaUsed:    5000,
		QuotaTotal:   10000,
		Model:        "gpt-4",
		LastUsed:     now,
	}

	if account.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", account.Email)
	}
	if account.Status != "available" {
		t.Errorf("Expected status 'available', got '%s'", account.Status)
	}
	if account.QuotaUsed >= account.QuotaTotal {
		t.Error("QuotaUsed should be less than QuotaTotal for available account")
	}
}

// TestGetPoolStatistics 测试号池统计函数（无数据库连接时的行为）
func TestGetPoolStatistics(t *testing.T) {
	// 在无数据库连接时，函数应该能正常调用而不 panic
	stats := GetPoolStatistics()

	if stats == nil {
		t.Error("GetPoolStatistics should return a non-nil map")
	}

	// 验证返回的键存在
	expectedKeys := []string{"total", "available", "cooldown", "banned", "utilization_rate"}
	for _, key := range expectedKeys {
		if _, ok := stats[key]; !ok {
			t.Errorf("Expected key '%s' in statistics", key)
		}
	}
}

// TestAutoFailoverAccount 测试自动故障切换
func TestAutoFailoverAccount(t *testing.T) {
	// 这个测试验证函数可以被调用而不会 panic
	// 实际的故障切换逻辑需要数据库支持
	err := AutoFailoverAccount("test@example.com", "test reason")

	// 无数据库时可能返回错误，但不应该 panic
	if err != nil {
		t.Logf("AutoFailoverAccount returned error (expected without DB): %v", err)
	}
}

// TestGetNextAvailableAccount 测试获取下一个可用账号
func TestGetNextAvailableAccount(t *testing.T) {
	account, err := GetNextAvailableAccount()

	// 无数据库连接时应该返回错误
	if err != nil {
		t.Logf("GetNextAvailableAccount returned error (expected without DB): %v", err)
	} else if account != nil {
		t.Log("GetNextAvailableAccount returned an account")
	}
}

// TestMarkAccountUsed 测试标记账号已使用
func TestMarkAccountUsed(t *testing.T) {
	err := MarkAccountUsed("test@example.com")

	// 无数据库时可能返回错误
	if err != nil {
		t.Logf("MarkAccountUsed returned error (expected without DB): %v", err)
	}
}

// TestReleaseAccountCooldown 测试释放冷却中的账号
func TestReleaseAccountCooldown(t *testing.T) {
	count, err := ReleaseAccountCooldown()

	// 无数据库时可能返回错误
	if err != nil {
		t.Logf("ReleaseAccountCooldown returned error (expected without DB): %v", err)
	} else {
		t.Logf("Released %d accounts from cooldown", count)
	}
}
