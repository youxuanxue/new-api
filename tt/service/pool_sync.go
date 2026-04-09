//go:build tt
// +build tt

package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

type PoolSyncConfig struct {
	SyncInterval     time.Duration
	RefreshBefore    time.Duration
	BanCheckInterval time.Duration
	MaxRetryCount    int
	CooldownDuration time.Duration
}

var DefaultPoolSyncConfig = PoolSyncConfig{
	SyncInterval:     5 * time.Minute,
	RefreshBefore:    30 * time.Minute,
	BanCheckInterval: 1 * time.Minute,
	MaxRetryCount:    3,
	CooldownDuration: 30 * time.Minute,
}

var currentPoolSyncConfig = DefaultPoolSyncConfig
var poolSyncConfigMutex sync.RWMutex

func SetPoolSyncConfig(config PoolSyncConfig) {
	poolSyncConfigMutex.Lock()
	defer poolSyncConfigMutex.Unlock()
	currentPoolSyncConfig = config
}

func GetPoolSyncConfig() PoolSyncConfig {
	poolSyncConfigMutex.RLock()
	defer poolSyncConfigMutex.RUnlock()
	return currentPoolSyncConfig
}

func SyncPoolAccounts(ctx context.Context, client *Sub2APIClient) (*SyncResult, error) {
	result := &SyncResult{}
	remoteAccounts, err := client.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	localAccounts, err := ttmodel.ListPoolAccounts("")
	if err != nil {
		return nil, err
	}
	localMap := make(map[string]interface{})
	for _, acc := range localAccounts {
		if account, ok := acc.(ttmodel.PoolAccount); ok {
			localMap[account.Email] = account
		}
	}
	remoteSet := make(map[string]bool)
	for _, acc := range remoteAccounts {
		remoteSet[acc.Email] = true
		if localAcc, exists := localMap[acc.Email]; exists {
			if err := updateLocalAccount(localAcc.(ttmodel.PoolAccount), acc); err != nil {
				result.Errors++
				logger.LogError(nil, fmt.Sprintf("[PoolSync] Failed to update account %s: %v", acc.Email, err))
			} else {
				result.Updated++
			}
		} else {
			if err := addLocalAccount(acc); err != nil {
				result.Errors++
				logger.LogError(nil, fmt.Sprintf("[PoolSync] Failed to add account %s: %v", acc.Email, err))
			} else {
				result.Added++
			}
		}
	}
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

func updateLocalAccount(local ttmodel.PoolAccount, remote AccountInfo) error {
	updates := make(map[string]interface{})
	if remote.AccessToken != "" {
		updates["oauth_token"] = remote.AccessToken
	}
	if remote.Status != "" && remote.Status != local.Status {
		updates["status"] = remote.Status
	}
	if remote.QuotaUsed > 0 {
		updates["quota_used"] = decimal.NewFromInt(remote.QuotaUsed).String()
	}
	if remote.QuotaTotal > 0 {
		updates["quota_total"] = decimal.NewFromInt(remote.QuotaTotal).String()
	}
	if !remote.LastUsed.IsZero() {
		updates["last_used"] = remote.LastUsed
	}
	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		return ttmodel.DB.Model(&local).Updates(updates).Error
	}
	return nil
}

func addLocalAccount(acc AccountInfo) error {
	account := ttmodel.PoolAccount{
		Email:      acc.Email,
		OAuthToken: acc.AccessToken,
		Status:     acc.Status,
		QuotaUsed:  decimal.NewFromInt(acc.QuotaUsed).String(),
		QuotaTotal: decimal.NewFromInt(acc.QuotaTotal).String(),
	}
	if !acc.LastUsed.IsZero() {
		account.LastUsed = &acc.LastUsed
	}
	return ttmodel.DB.Create(&account).Error
}

func RefreshExpiredTokens(ctx context.Context, client *Sub2APIClient) (int, int) {
	config := GetPoolSyncConfig()
	refreshBefore := time.Now().Add(config.RefreshBefore)
	var accounts []ttmodel.PoolAccount
	err := ttmodel.DB.Where("status = ? AND oauth_token IS NOT NULL AND oauth_token != ''", "available").Find(&accounts).Error
	if err != nil {
		logger.LogError(nil, fmt.Sprintf("[TokenRefresh] Failed to query accounts: %v", err))
		return 0, 0
	}
	successCount, failCount := 0, 0
	for _, acc := range accounts {
		if acc.LastUsed != nil && acc.LastUsed.Before(refreshBefore) {
			result, err := client.RefreshToken(ctx, acc.Email)
			if err != nil {
				failCount++
				logger.LogError(nil, fmt.Sprintf("[TokenRefresh] Failed to refresh token for %s: %v", acc.Email, err))
				continue
			}
			if result.Success {
				updates := map[string]interface{}{"oauth_token": result.AccessToken, "updated_at": time.Now(), "request_count": 0}
				ttmodel.DB.Model(&acc).Updates(updates)
				successCount++
			} else {
				failCount++
				logger.LogError(nil, fmt.Sprintf("[TokenRefresh] Token refresh failed for %s: %s", acc.Email, result.Error))
			}
		}
	}
	return successCount, failCount
}

func DetectBannedAccounts(ctx context.Context, client *Sub2APIClient) ([]string, error) {
	var accounts []ttmodel.PoolAccount
	err := ttmodel.DB.Where("status IN ?", []string{"available", "cooldown"}).Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, nil
	}
	emails := make([]string, len(accounts))
	for i, acc := range accounts {
		emails[i] = acc.Email
	}
	statuses, err := client.BatchCheckBan(ctx, emails)
	if err != nil {
		return nil, err
	}
	var bannedEmails []string
	for _, status := range statuses {
		if status.IsBanned {
			bannedEmails = append(bannedEmails, status.Email)
			ttmodel.DB.Model(&ttmodel.PoolAccount{}).Where("email = ?", status.Email).Updates(map[string]interface{}{
				"status": "banned", "cooldown_end": nil, "updated_at": time.Now(),
			})
		}
	}
	return bannedEmails, nil
}

func AutoFailoverAccount(email string, reason string) error {
	config := GetPoolSyncConfig()
	cooldownEnd := time.Now().Add(config.CooldownDuration)
	err := ttmodel.DB.Model(&ttmodel.PoolAccount{}).Where("email = ?", email).Updates(map[string]interface{}{
		"status": "cooldown", "cooldown_end": cooldownEnd, "updated_at": time.Now(),
	}).Error
	if err != nil {
		return err
	}
	var nextAccount ttmodel.PoolAccount
	err = ttmodel.DB.Where("status = ? AND oauth_token IS NOT NULL AND oauth_token != ''", "available").Order("request_count ASC, last_used ASC").First(&nextAccount).Error
	if err != nil {
		return nil
	}
	return nil
}

func GetNextAvailableAccount() (*ttmodel.PoolAccount, error) {
	var account ttmodel.PoolAccount
	err := ttmodel.DB.Where("status = ? AND oauth_token IS NOT NULL AND oauth_token != ''", "available").Order("request_count ASC, last_used ASC").First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func MarkAccountUsed(email string) error {
	now := time.Now()
	return ttmodel.DB.Model(&ttmodel.PoolAccount{}).Where("email = ?", email).Updates(map[string]interface{}{
		"last_used": now, "request_count": ttmodel.DB.Raw("request_count + 1"), "updated_at": now,
	}).Error
}

func ReleaseAccountCooldown() (int, error) {
	now := time.Now()
	result := ttmodel.DB.Model(&ttmodel.PoolAccount{}).Where("status = ? AND cooldown_end IS NOT NULL AND cooldown_end <= ?", "cooldown", now).Updates(map[string]interface{}{
		"status": "available", "cooldown_end": nil, "updated_at": now,
	})
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

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
	return map[string]interface{}{"total": total, "available": available, "cooldown": cooldown, "banned": banned, "utilization_rate": utilizationRate}
}

func StartBanDetectionTask() {
	StartBanDetectionTaskWithContext(context.Background())
}

func StartBanDetectionTaskWithContext(ctx context.Context) {
	logger.LogInfo(nil, "[BanDetect] Starting ban detection task...")
	config := GetPoolSyncConfig()
	ticker := time.NewTicker(config.BanCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.LogInfo(nil, "[BanDetect] Ban detection task stopped")
			return
		case <-ticker.C:
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
				logger.LogError(nil, fmt.Sprintf("[BanDetect] Detect banned accounts failed: %v", err))
				common.SendFeishuAlert(
					"Pool ban detection failed",
					fmt.Sprintf("ban detection error: %v", err),
					common.AlertWarning,
				)
			} else if len(banned) > 0 {
				common.SendFeishuAlert(
					"Pool accounts banned",
					fmt.Sprintf("%d pool accounts moved to banned state", len(banned)),
					common.AlertWarning,
				)
			}
			released, releaseErr := ReleaseAccountCooldown()
			if releaseErr != nil {
				logger.LogError(nil, fmt.Sprintf("[BanDetect] Release cooldown failed: %v", releaseErr))
			} else if released > 0 {
				logger.LogInfo(nil, fmt.Sprintf("[BanDetect] Released %d cooldown accounts", released))
			}
		}()
		}
	}
}
