//go:build tt
// +build tt

package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/tt/hooks"
	ttservice "github.com/QuantumNous/new-api/tt/service"
)

var stopTTTasks context.CancelFunc = func() {}

func startTTBootstrapTasks() {
	taskCtx, cancel := context.WithCancel(context.Background())
	stopTTTasks = cancel

	hooks.InitHooks()
	common.InitFeishuAlert()
	ttservice.SetSub2APIConfig(ttservice.Sub2APIConfig{
		BaseURL: envOrDefault("SUB2API_URL", ttservice.DefaultSub2APIConfig.BaseURL),
		APIKey:  os.Getenv("SUB2API_KEY"),
		Timeout: time.Duration(envIntOrDefault("SUB2API_TIMEOUT_SECONDS", int(ttservice.DefaultSub2APIConfig.Timeout.Seconds()))) * time.Second,
	})
	ttservice.SetPoolSyncConfig(ttservice.PoolSyncConfig{
		SyncInterval:     time.Duration(envIntOrDefault("POOL_SYNC_INTERVAL", int(ttservice.DefaultPoolSyncConfig.SyncInterval.Minutes()))) * time.Minute,
		RefreshBefore:    time.Duration(envIntOrDefault("TOKEN_REFRESH_BEFORE", int(ttservice.DefaultPoolSyncConfig.RefreshBefore.Minutes()))) * time.Minute,
		BanCheckInterval: ttservice.DefaultPoolSyncConfig.BanCheckInterval,
		MaxRetryCount:    ttservice.DefaultPoolSyncConfig.MaxRetryCount,
		CooldownDuration: ttservice.DefaultPoolSyncConfig.CooldownDuration,
	})
	go func() {
		time.Sleep(5 * time.Second)
		ttservice.StartPoolSyncTaskWithContext(taskCtx)
	}()
	go func() {
		time.Sleep(10 * time.Second)
		ttservice.StartBanDetectionTaskWithContext(taskCtx)
	}()
}

func stopTTBootstrapTasks() {
	stopTTTasks()
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envIntOrDefault(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}
