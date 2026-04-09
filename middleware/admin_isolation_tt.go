//go:build tt
// +build tt

package middleware

import (
	"log"
	"os"

	"github.com/QuantumNous/new-api/common"
	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

var adminIPWhitelist []string

func SetAdminIPWhitelist(ips []string) {
	adminIPWhitelist = ips
}

func GetAdminIPWhitelist() []string {
	return adminIPWhitelist
}

func InitAdminIsolation() error {
	var err error
	adminAuditFile, err = os.OpenFile("/var/log/tt/admin_audit.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		log.Printf("[WARN] Failed to open admin audit log: %v", err)
	}
	return nil
}

// AdminAuthBridge converts the standard auth context (id/username/role set by
// AdminAuth) into the *AdminUser struct that AdminIsolation expects.
func AdminAuthBridge() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := c.Get("id")
		username, _ := c.Get("username")
		roleVal, _ := c.Get("role")

		if id == nil || username == nil || roleVal == nil {
			c.Next()
			return
		}

		adminID, ok := toInt(id)
		if !ok || adminID <= 0 {
			c.Next()
			return
		}
		usernameStr, ok := username.(string)
		if !ok || usernameStr == "" {
			c.Next()
			return
		}
		roleInt, ok := toInt(roleVal)
		if !ok {
			c.Next()
			return
		}

		// Root users (role=100) are always super_admin.
		// Admin users (role=10) get their TT role from UserExtension.AdminRole.
		var adminRole AdminRole
		if roleInt >= common.RoleRootUser {
			adminRole = RoleSuperAdmin
		} else {
			stored := ttmodel.GetTTAdminRole(adminID)
			adminRole = AdminRole(stored)
		}

		user, err := ttmodel.GetUserById(adminID, false)
		if err != nil || user == nil {
			c.Next()
			return
		}

		var totpSecret string
		twoFA, err := ttmodel.GetTwoFAByUserId(adminID)
		if err == nil && twoFA != nil && twoFA.IsEnabled {
			totpSecret = twoFA.Secret
		}

		admin := &AdminUser{
			ID:         uint(adminID),
			Username:   usernameStr,
			Role:       adminRole,
			TOTPSecret: totpSecret,
			IsActive:   user.Status == common.UserStatusEnabled,
		}
		c.Set("admin_user", admin)
		c.Set("admin_id", adminID)
		c.Next()
	}
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
