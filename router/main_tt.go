//go:build tt
// +build tt

package router

import (
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// initTT registers TT-specific routes and middleware.
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
