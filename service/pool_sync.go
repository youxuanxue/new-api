//go:build tt
// +build tt

package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/logger"
	ttmodel "github.com/QuantumNous/new-api/model"

	"github.com/shopspring/decimal"
)

// PoolSyncConfig 账号池同步配置
type PoolSyncConfig struct {
	SyncInterval      time.Duration // 同步间隔
	RefreshBefore     time.Duration // Token 过期前多久刷新
	BanCheckInterval  time.Duration // 封号检查间隔
	MaxRetryCount     int           // 最大重试次数
	CooldownDuration  time.Duration // 冷却时长
}

// DefaultPoolSyncConfig 默认配置
var DefaultPoolSyncConfig = PoolSyncConfig{
	SyncInterval:      5 * time.Minute,
	RefreshBefore:     30 * time.Minute,
	BanCheckInterval:  1 * time.Minute,
	MaxRetryCount:     3,
	CooldownDuration:  30 * time.Minute,
}

// currentPoolSyncConfig 当前配置
var currentPoolSyncConfig = DefaultPoolSyncConfig
var poolSyncConfigMutex sync.RWMutex

// SetPoolSyncConfig 设置配置
func SetPoolSyncConfig(config PoolSyncConfig) {
	poolSyncConfigMutex.Lock()
	defer poolSyncConfigMutex.Unlock()
	currentPoolSyncConfig = config
}

// GetPoolSyncConfig 获取配置
func GetPoolSyncConfig() PoolSyncConfig {
	poolSyncConfigMutex.RLock()
	defer poolSyncConfigMutex.RUnlock()
	return currentPoolSyncConfig
}

// SyncPoolAccounts 同步账号池
func SyncPoolAccounts(ctx context.Context, client *Sub2APIClient) (*SyncResult, error) {
	result := &SyncResult{}

	// 获取远程账号列表
	remoteAccounts, err := client.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}

	// 获取本地账号列表
	localAccounts, err := ttmodel.ListPoolAccounts("")
	if err != nil {
		return nil, err
	}

	// 构建本地账号映射
	localMap := make(map[string]interface{})
	for _, acc := range localAccounts {
		if account, ok := acc.(ttmodel.PoolAccount); ok {
			localMap[account.Email] = account
		}
	}

	// 构建远程账号集合
	remoteSet := make(map[string]bool)
	for _, acc := range remoteAccounts {
		remoteSet[acc.Email] = true

		// 检查是否需要添加或更新
		if localAcc, exists := localMap[acc.Email]; exists {
			// 更新现有账号
			if err := updateLocalAccount(localAcc.(ttmodel.PoolAccount), acc); err != nil {
				result.Errors++
				logger.LogError(nil, fmt.Sprintf("[PoolSync] Failed to update account %s: %v", acc.Email, err))
			} else {
				result.Updated++
			}
		} else {
			// 添加新账号
			if err := addLocalAccount(acc); err != nil {
				result.Errors++
				logger.LogError(nil, fmt.Sprintf("[PoolSync] Failed to add account %s: %v", acc.Email, err))
			} else {
				result.Added++
			}
		}
	}

	// 检查需要移除的账号
	for email := range localMap {
		if !remoteSet[email] {
			if err := ttmodel.RemovePoolAccountByEmail(email); err != nil {
				result.Errors++
				logger.LogError(nil, fmt.Sprintf("[PoolSync] Failed to remove account %s: %v", email, err))
			} else {
				result.Removed++
			}
		}
	}

	return result, nil
}

// updateLocalAccount 更新本地账号
func updateLocalAccount(local ttmodel.PoolAccount, remote AccountInfo) error {
	updates := make(map[string]interface{})

	// 更新 Token
	if remote.AccessToken != "" {
		updates["oauth_token"] = remote.AccessToken
	}

	// 更新状态
	if remote.Status != "" && remote.Status != local.Status {
		updates["status"] = remote.Status
	}

	// 更新配额
	if remote.QuotaUsed > 0 {
		updates["quota_used"] = decimal.NewFromInt(remote.QuotaUsed).String()
	}
	if remote.QuotaTotal > 0 {
		updates["quota_total"] = decimal.NewFromInt(remote.QuotaTotal).String()
	}

	// 更新最后使用时间
	if !remote.LastUsed.IsZero() {
		updates["last_used"] = remote.LastUsed
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		return ttmodel.DB.Model(&local).Updates(updates).Error
	}

	return nil
}

// addLocalAccount 添加本地账号
func addLocalAccount(acc AccountInfo) error {
	account := ttmodel.PoolAccount{
		Email:       acc.Email,
		OAuthToken:  acc.AccessToken,
		Status:      acc.Status,
		QuotaUsed:   decimal.NewFromInt(acc.QuotaUsed).String(),
		QuotaTotal:  decimal.NewFromInt(acc.QuotaTotal).String(),
	}

	if !acc.LastUsed.IsZero() {
		account.LastUsed = &acc.LastUsed
	}

	return ttmodel.DB.Create(&account).Error
}

// RefreshExpiredTokens 刷新即将过期的 Token
func RefreshExpiredTokens(ctx context.Context, client *Sub2APIClient) (int, int) {
	config := GetPoolSyncConfig()
	refreshBefore := time.Now().Add(config.RefreshBefore)

	// 查询需要刷新的账号（Token 即将过期）
	var accounts []ttmodel.PoolAccount
	err := ttmodel.DB.Where("status = ? AND oauth_token IS NOT NULL AND oauth_token != ''", "available").
		Find(&accounts).Error
	if err != nil {
		logger.LogError(nil, fmt.Sprintf("[TokenRefresh] Failed to query accounts: %v", err))
		return 0, 0
	}

	successCount := 0
	failCount := 0

	for _, acc := range accounts {
		// 检查是否需要刷新（基于上次使用时间判断）
		if acc.LastUsed != nil && acc.LastUsed.Before(refreshBefore) {
			result, err := client.RefreshToken(ctx, acc.Email)
			if err != nil {
				failCount++
				logger.LogError(nil, fmt.Sprintf("[TokenRefresh] Failed to refresh token for %s: %v", acc.Email, err))
				continue
			}

			if result.Success {
				// 更新 Token
				updates := map[string]interface{}{
					"oauth_token":  result.AccessToken,
					"updated_at":   time.Now(),
					"request_count": 0, // 重置请求计数
				}
				ttmodel.DB.Model(&acc).Updates(updates)
				successCount++
				logger.LogInfo(nil, fmt.Sprintf("[TokenRefresh] Token refreshed for %s", acc.Email))
			} else {
				failCount++
				logger.LogError(nil, fmt.Sprintf("[TokenRefresh] Token refresh failed for %s: %s", acc.Email, result.Error))
			}
		}
	}

	return successCount, failCount
}

// DetectBannedAccounts 检测被封账号
func DetectBannedAccounts(ctx context.Context, client *Sub2APIClient) ([]string, error) {
	// 获取所有可用账号
	var accounts []ttmodel.PoolAccount
	err := ttmodel.DB.Where("status IN ?", []string{"available", "cooldown"}).Find(&accounts).Error
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, nil
	}

	// 提取邮箱列表
	emails := make([]string, len(accounts))
	for i, acc := range accounts {
		emails[i] = acc.Email
	}

	// 批量检查封号状态
	statuses, err := client.BatchCheckBan(ctx, emails)
	if err != nil {
		return nil, err
	}

	var bannedEmails []string

	// 处理检测结果
	for _, status := range statuses {
		if status.IsBanned {
			bannedEmails = append(bannedEmails, status.Email)
			// 更新本地状态
			ttmodel.DB.Model(&ttmodel.PoolAccount{}).
				Where("email = ?", status.Email).
				Updates(map[string]interface{}{
					"status":       "banned",
					"cooldown_end": nil,
					"updated_at":   time.Now(),
				})
			logger.LogInfo(nil, fmt.Sprintf("[BanDetect] Account %s detected as banned: %s", status.Email, status.BanReason))
		}
	}

	return bannedEmails, nil
}

// AutoFailoverAccount 自动故障切换账号
func AutoFailoverAccount(email string, reason string) error {
	// 将当前账号标记为冷却
	config := GetPoolSyncConfig()
	cooldownEnd := time.Now().Add(config.CooldownDuration)

	err := ttmodel.DB.Model(&ttmodel.PoolAccount{}).
		Where("email = ?", email).
		Updates(map[string]interface{}{
			"status":       "cooldown",
			"cooldown_end": cooldownEnd,
			"updated_at":   time.Now(),
		}).Error

	if err != nil {
		return err
	}

	logger.LogInfo(nil, fmt.Sprintf("[Failover] Account %s set to cooldown for %v, reason: %s", email, config.CooldownDuration, reason))

	// 尝试获取下一个可用账号
	var nextAccount ttmodel.PoolAccount
	err = ttmodel.DB.Where("status = ? AND oauth_token IS NOT NULL AND oauth_token != ''", "available").
		Order("request_count ASC, last_used ASC").
		First(&nextAccount).Error

	if err != nil {
		logger.LogError(nil, "[Failover] No available account for failover")
		return nil // 没有可用账号不报错
	}

	logger.LogInfo(nil, fmt.Sprintf("[Failover] Switched to account %s", nextAccount.Email))
	return nil
}

// GetNextAvailableAccount 获取下一个可用账号
func GetNextAvailableAccount() (*ttmodel.PoolAccount, error) {
	var account ttmodel.PoolAccount
	err := ttmodel.DB.Where("status = ? AND oauth_token IS NOT NULL AND oauth_token != ''", "available").
		Order("request_count ASC, last_used ASC").
		First(&account).Error

	if err != nil {
		return nil, err
	}

	return &account, nil
}

// MarkAccountUsed 标记账号已使用
func MarkAccountUsed(email string) error {
	now := time.Now()
	return ttmodel.DB.Model(&ttmodel.PoolAccount{}).
		Where("email = ?", email).
		Updates(map[string]interface{}{
			"last_used":     now,
			"request_count": ttmodel.DB.Raw("request_count + 1"),
			"updated_at":    now,
		}).Error
}

// ReleaseAccountCooldown 释放冷却中的账号
func ReleaseAccountCooldown() (int, error) {
	now := time.Now()
	result := ttmodel.DB.Model(&ttmodel.PoolAccount{}).
		Where("status = ? AND cooldown_end IS NOT NULL AND cooldown_end <= ?", "cooldown", now).
		Updates(map[string]interface{}{
			"status":       "available",
			"cooldown_end": nil,
			"updated_at":   now,
		})

	if result.Error != nil {
		return 0, result.Error
	}

	return int(result.RowsAffected), nil
}

// GetPoolStatistics 获取号池统计
func GetPoolStatistics() map[string]interface{} {
	var total, available, cooldown, banned int64

	ttmodel.DB.Model(&ttmodel.PoolAccount{}).Count(&total)
	ttmodel.DB.Model(&ttmodel.PoolAccount{}).Where("status = ?", "available").Count(&available)
	ttmodel.DB.Model(&ttmodel.PoolAccount{}).Where("status = ?", "cooldown").Count(&cooldown)
	ttmodel.DB.Model(&ttmodel.PoolAccount{}).Where("status = ?", "banned").Count(&banned)

	utilizationRate := 0.0
	if total > 0 {
		utilizationRate = float64(available) / float64(total) * 100
	}

	return map[string]interface{}{
		"total":           total,
		"available":       available,
		"cooldown":        cooldown,
		"banned":          banned,
		"utilization_rate": utilizationRate,
	}
}

// StartBanDetectionTask 启动封号检测任务
func StartBanDetectionTask() {
	logger.LogInfo(nil, "[BanDetect] Starting ban detection task...")

	config := GetPoolSyncConfig()
	ticker := time.NewTicker(config.BanCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			sub2apiConfig := GetSub2APIConfig()
			if sub2apiConfig.APIKey == "" {
				return
			}

			client := NewSub2APIClient(sub2apiConfig.BaseURL, sub2apiConfig.APIKey)
			banned, err := DetectBannedAccounts(ctx, client)
			if err != nil {
				logger.LogError(nil, fmt.Sprintf("[BanDetect] Detection failed: %v", err))
				return
			}

			if len(banned) > 0 {
				logger.LogInfo(nil, fmt.Sprintf("[BanDetect] Detected %d banned accounts", len(banned)))
			}

			// 释放冷却完成的账号
			released, err := ReleaseAccountCooldown()
			if err != nil {
				logger.LogError(nil, fmt.Sprintf("[BanDetect] Failed to release cooldown accounts: %v", err))
			} else if released > 0 {
				logger.LogInfo(nil, fmt.Sprintf("[BanDetect] Released %d accounts from cooldown", released))
			}
		}()
	}
}
