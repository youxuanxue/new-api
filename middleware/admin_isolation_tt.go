//go:build tt
// +build tt

package middleware

import (
	"log"
	"os"

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

		var adminRole AdminRole
		switch roleVal.(int) {
		case 100:
			adminRole = RoleSuperAdmin
		case 10:
			adminRole = RoleOperator
		default:
			adminRole = RoleViewer
		}

		admin := &AdminUser{
			ID:       uint(id.(int)),
			Username: username.(string),
			Role:     adminRole,
			IsActive: true,
		}
		c.Set("admin_user", admin)
		c.Next()
	}
}
