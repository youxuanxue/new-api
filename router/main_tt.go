//go:build tt
// +build tt

// Package router provides HTTP routing
// main_tt.go - TT-specific router initialization (only included in TT builds)
package router

import (
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// initTT registers TT-specific routes
func initTT(router *gin.Engine) {
	if err := middleware.InitSecurityProxy(); err != nil {
		common.SysError("InitSecurityProxy failed: " + err.Error())
	} else {
		router.Use(middleware.SecurityProxy())
	}

	if err := middleware.InitAdminIsolation(); err != nil {
		common.SysError("InitAdminIsolation failed: " + err.Error())
	}

	if ipList := os.Getenv("ADMIN_IP_WHITELIST"); ipList != "" {
		ips := strings.Split(ipList, ",")
		for i := range ips {
			ips[i] = strings.TrimSpace(ips[i])
		}
		middleware.SetAdminIPWhitelist(ips)
	}

	SetTTApiRouter(router)
	SetTTAdminRouter(router)
	SetTTPublicRouter(router)
}
