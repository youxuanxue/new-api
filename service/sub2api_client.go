//go:build tt
// +build tt

package service

import ttservice "github.com/QuantumNous/new-api/tt/service"

type Sub2APIConfig = ttservice.Sub2APIConfig
type Sub2APIClient = ttservice.Sub2APIClient
type AccountInfo = ttservice.AccountInfo
type SyncResult = ttservice.SyncResult
type RefreshResult = ttservice.RefreshResult
type BanStatus = ttservice.BanStatus

var DefaultSub2APIConfig = ttservice.DefaultSub2APIConfig

func SetSub2APIConfig(config Sub2APIConfig) {
	ttservice.SetSub2APIConfig(config)
}

func GetSub2APIConfig() Sub2APIConfig {
	return ttservice.GetSub2APIConfig()
}

func NewSub2APIClient(baseURL, apiKey string) *Sub2APIClient {
	return ttservice.NewSub2APIClient(baseURL, apiKey)
}

func StartPoolSyncTask() {
	ttservice.StartPoolSyncTask()
}
