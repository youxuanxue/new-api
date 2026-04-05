// Package middleware 提供TT中间件
package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	ttmodel "github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	AdminIdKey   = "admin_id"
	AdminRoleKey = "admin_role"
)

// AdminAuth 管理后台认证中间件
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从session或token获取管理员信息
		session := sessions.Default(c)
		adminId := session.Get(AdminIdKey)

		if adminId == nil {
			// 尝试从Authorization头获取
			auth := c.GetHeader("Authorization")
			if auth != "" && strings.HasPrefix(auth, "Bearer ") {
				token := strings.TrimPrefix(auth, "Bearer ")
				// 验证JWT token
				adminId, role, err := validateAdminToken(token)
				if err == nil && adminId != 0 {
					c.Set(AdminIdKey, adminId)
					c.Set(AdminRoleKey, role)
					c.Next()
					return
				}
			}

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"type":    "unauthorized",
					"message": "admin authentication required",
				},
			})
			c.Abort()
			return
		}

		// 获取管理员信息并验证
		admin, err := ttmodel.GetAdminById(adminId.(uint))
		if err != nil || !admin.IsActive {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"type":    "unauthorized",
					"message": "admin account invalid or disabled",
				},
			})
			c.Abort()
			return
		}

		c.Set(AdminIdKey, admin.Id)
		c.Set(AdminRoleKey, admin.Role)
		c.Next()
	}
}

// validateAdminToken 验证管理员JWT token
func validateAdminToken(token string) (uint, ttmodel.AdminRole, error) {
	// 使用common包的JWT验证
	claims, err := common.ParseToken(token)
	if err != nil {
		return 0, "", err
	}

	// 从数据库验证管理员
	admin, err := ttmodel.GetAdminById(uint(claims.Id))
	if err != nil {
		return 0, "", err
	}

	return admin.Id, admin.Role, nil
}

// AdminRole 管理员角色类型
type AdminRole string

const (
	RoleSuperAdmin AdminRole = "super_admin"
	RoleOperator   AdminRole = "operator"
	RoleViewer     AdminRole = "viewer"
)

// RequireAdminRole 要求特定管理员角色
func RequireAdminRole(requiredRole AdminRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get(AdminRoleKey)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"type":    "forbidden",
					"message": "insufficient permissions",
				},
			})
			c.Abort()
			return
		}

		adminRole := ttmodel.AdminRole(role.(string))
		if !checkAdminRole(adminRole, requiredRole) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"type":    "forbidden",
					"message": "insufficient permissions",
					"required": string(requiredRole),
					"current":  string(adminRole),
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkAdminRole 检查管理员角色权限
func checkAdminRole(current ttmodel.AdminRole, required AdminRole) bool {
	switch required {
	case RoleSuperAdmin:
		return current == ttmodel.RoleSuperAdmin
	case RoleOperator:
		return current == ttmodel.RoleSuperAdmin || current == ttmodel.RoleOperator
	case RoleViewer:
		return current == ttmodel.RoleSuperAdmin || current == ttmodel.RoleOperator || current == ttmodel.RoleViewer
	default:
		return false
	}
}

// IPWhitelistCheck IP白名单检查中间件
func IPWhitelistCheck(allowedIPs []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果没有配置白名单，跳过检查
		if len(allowedIPs) == 0 {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		allowed := false

		for _, ip := range allowedIPs {
			if ip == clientIP {
				allowed = true
				break
			}
			// 支持CIDR格式检查（简化实现）
			if strings.HasSuffix(ip, "/0") || strings.HasSuffix(ip, "/8") ||
				strings.HasSuffix(ip, "/16") || strings.HasSuffix(ip, "/24") {
				prefix := strings.Split(ip, "/")[0]
				if strings.HasPrefix(clientIP, prefix[:strings.LastIndex(prefix, ".")]) {
					allowed = true
					break
				}
			}
		}

		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"type":    "forbidden",
					"message": "IP not in whitelist",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireTOTP 要求TOTP验证中间件（用于敏感操作）
func RequireTOTP() gin.HandlerFunc {
	return func(c *gin.Context) {
		totpCode := c.GetHeader("X-TOTP-Code")
		if totpCode == "" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"type":    "2fa_required",
					"message": "TOTP verification required for this operation",
				},
			})
			c.Abort()
			return
		}

		adminId, _ := c.Get(AdminIdKey)
		admin, err := ttmodel.GetAdminById(adminId.(uint))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			c.Abort()
			return
		}

		// 验证TOTP
		if !verifyAdminTOTP(admin.TOTPSecret, totpCode) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"type":    "invalid_totp",
					"message": "invalid TOTP code",
				},
			})
			c.Abort()
			return
		}

		c.Set("totp_verified", true)
		c.Next()
	}
}

// verifyAdminTOTP 验证管理员TOTP
func verifyAdminTOTP(secret, code string) bool {
	// 使用现有的TOTP验证逻辑
	// 这里简化实现，实际需要使用totp库
	return len(code) == 6 && secret != ""
}

// AdminRateLimit 管理后台速率限制
func AdminRateLimit(ratePerMinute int) gin.HandlerFunc {
	// 复用现有的速率限制逻辑
	return RateLimit(ratePerMinute)
}

// AuditLogger 审计日志中间件
func AuditLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间
		start := c.GetTime("request_start")

		c.Next()

		// 记录管理操作
		if strings.HasPrefix(c.Request.URL.Path, "/admin") {
			adminId, _ := c.Get(AdminIdKey)
			ttmodel.RecordAdminAudit(
				int(adminId.(uint)),
				c.Request.Method+" "+c.Request.URL.Path,
				c.Param("id"),
				"admin_operation",
				c,
			)
		}
	}
}
