//go:build tt
// +build tt

package service

import (
	"context"
	ttservice "github.com/QuantumNous/new-api/tt/service"
	ttmodel "github.com/QuantumNous/new-api/model"
)

type PoolSyncConfig = ttservice.PoolSyncConfig

var DefaultPoolSyncConfig = ttservice.DefaultPoolSyncConfig

func SetPoolSyncConfig(config PoolSyncConfig) {
	ttservice.SetPoolSyncConfig(config)
}

func GetPoolSyncConfig() PoolSyncConfig {
	return ttservice.GetPoolSyncConfig()
}

func SyncPoolAccounts(ctx context.Context, client *Sub2APIClient) (*SyncResult, error) {
	return ttservice.SyncPoolAccounts(ctx, client)
}

func RefreshExpiredTokens(ctx context.Context, client *Sub2APIClient) (int, int) {
	return ttservice.RefreshExpiredTokens(ctx, client)
}

func DetectBannedAccounts(ctx context.Context, client *Sub2APIClient) ([]string, error) {
	return ttservice.DetectBannedAccounts(ctx, client)
}

func AutoFailoverAccount(email string, reason string) error {
	return ttservice.AutoFailoverAccount(email, reason)
}

func GetNextAvailableAccount() (*ttmodel.PoolAccount, error) {
	return ttservice.GetNextAvailableAccount()
}

func MarkAccountUsed(email string) error {
	return ttservice.MarkAccountUsed(email)
}

func ReleaseAccountCooldown() (int, error) {
	return ttservice.ReleaseAccountCooldown()
}

func GetPoolStatistics() map[string]interface{} {
	return ttservice.GetPoolStatistics()
}

func StartBanDetectionTask() {
	ttservice.StartBanDetectionTask()
}
