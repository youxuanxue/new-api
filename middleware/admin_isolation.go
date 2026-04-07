//go:build tt
// +build tt

// Package middleware 提供TT核心中间件
// admin_isolation.go - 管理后台隔离中间件，实现路由隔离、敏感操作审计、关键操作二次验证
package middleware

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// AdminRole 管理员角色
type AdminRole string

const (
	RoleSuperAdmin AdminRole = "super_admin"
	RoleOperator   AdminRole = "operator"
	RoleViewer     AdminRole = "viewer"
)

// AdminUser 管理员用户
type AdminUser struct {
	ID        uint
	Username  string
	Role      AdminRole
	TOTPSecret string
	IsActive  bool
}

// 敏感操作定义
var sensitiveOperations = map[string]bool{
	"PUT /admin/users":       true,
	"DELETE /admin/users":    true,
	"POST /admin/pricing":    true,
	"PUT /admin/pricing":     true,
	"DELETE /admin/channels": true,
	"PUT /admin/channels":    true,
	"POST /admin/users/balance": true,
}

// 关键操作定义（需要二次验证）
var criticalOperations = map[string]bool{
	"PUT /admin/pricing":          true,
	"POST /admin/users/balance":   true,
	"DELETE /admin/channels":      true,
	"PUT /admin/users/status":     true,
}

// 操作权限映射
var operationPermissions = map[string]AdminRole{
	// 用户管理
	"GET /admin/users":    RoleViewer,
	"PUT /admin/users":    RoleOperator,
	"DELETE /admin/users": RoleSuperAdmin,

	// 渠道管理
	"GET /admin/channels":    RoleViewer,
	"POST /admin/channels":   RoleOperator,
	"PUT /admin/channels":    RoleOperator,
	"DELETE /admin/channels": RoleSuperAdmin,

	// 定价管理
	"GET /admin/pricing":  RoleViewer,
	"POST /admin/pricing": RoleSuperAdmin,
	"PUT /admin/pricing":  RoleSuperAdmin,

	// 财务管理
	"GET /admin/finance":      RoleSuperAdmin,
	"POST /admin/users/balance": RoleSuperAdmin,

	// 系统设置
	"GET /admin/settings":  RoleOperator,
	"PUT /admin/settings":  RoleSuperAdmin,

	// 审计日志
	"GET /admin/audit": RoleSuperAdmin,
}

// AdminAuditLog 管理员审计日志
type AdminAuditLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AdminID     uint      `gorm:"index" json:"admin_id"`
	AdminName   string    `json:"admin_name"`
	Operation   string    `json:"operation"`
	TargetID    string    `json:"target_id,omitempty"`
	TargetType  string    `json:"target_type,omitempty"` // user/channel/pricing
	OldValue    string    `json:"old_value,omitempty"`
	NewValue    string    `json:"new_value,omitempty"`
	IP          string    `json:"ip"`
	UserAgent   string    `json:"user_agent"`
	TOTPVefified bool     `json:"totp_verified"`
	CreatedAt   time.Time `json:"created_at"`
}

// adminAuditFile 管理员审计日志文件
var (
	adminAuditFile *os.File
	auditMutex     sync.Mutex
)

// AdminIsolation 管理后台隔离中间件
func AdminIsolation() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 仅对 /admin 路由生效
		if !strings.HasPrefix(path, "/admin") {
			c.Next()
			return
		}

		// 1. 验证管理员身份
		adminUser, exists := c.Get("admin_user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
				"hint":  "admin authentication required",
			})
			return
		}

		admin := adminUser.(*AdminUser)
		if !admin.IsActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "account disabled",
			})
			return
		}

		// 2. 检查操作权限
		opKey := c.Request.Method + " " + path
		if !checkPermission(admin.Role, opKey) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":      "permission denied",
				"required":   string(getRequiredRole(opKey)),
				"current":    string(admin.Role),
			})
			return
		}

		// 3. 关键操作二次验证（TOTP）
		if criticalOperations[opKey] {
			totpCode := c.GetHeader("X-TOTP-Code")
			if totpCode == "" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "2FA required",
					"hint":  "Provide X-TOTP-Code header with valid TOTP code",
				})
				return
			}

			if !verifyTOTP(admin.TOTPSecret, totpCode) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "invalid TOTP code",
				})
				return
			}

			c.Set("totp_verified", true)
		}

		// 4. 敏感操作审计
		if sensitiveOperations[opKey] {
			go recordAdminAudit(c, admin, opKey)
		}

		c.Next()
	}
}

// checkPermission 检查权限
func checkPermission(role AdminRole, operation string) bool {
	requiredRole, exists := operationPermissions[operation]
	if !exists {
		// 未定义的操作，默认需要 SuperAdmin
		return role == RoleSuperAdmin
	}

	switch requiredRole {
	case RoleViewer:
		return role == RoleViewer || role == RoleOperator || role == RoleSuperAdmin
	case RoleOperator:
		return role == RoleOperator || role == RoleSuperAdmin
	case RoleSuperAdmin:
		return role == RoleSuperAdmin
	default:
		return false
	}
}

// getRequiredRole 获取操作所需角色
func getRequiredRole(operation string) AdminRole {
	if role, exists := operationPermissions[operation]; exists {
		return role
	}
	return RoleSuperAdmin
}

// verifyTOTP 验证 TOTP
func verifyTOTP(secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}

	valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    6,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		log.Printf("[ERROR] TOTP validation error: %v", err)
		return false
	}
	return valid
}

// recordAdminAudit 记录管理员审计日志
func recordAdminAudit(c *gin.Context, admin *AdminUser, operation string) {
	audit := AdminAuditLog{
		AdminID:     admin.ID,
		AdminName:   admin.Username,
		Operation:   operation,
		TargetID:    c.Param("id"),
		IP:          c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
		TOTPVefified: c.GetBool("totp_verified"),
		CreatedAt:   time.Now(),
	}

	// 提取目标类型
	parts := strings.Split(c.Request.URL.Path, "/")
	if len(parts) >= 3 {
		audit.TargetType = parts[2] // /admin/users/123 -> users
	}

	// 提取变更前后值（如果有）
	if oldValue, exists := c.Get("old_value"); exists {
		audit.OldValue = fmt.Sprintf("%v", oldValue)
	}
	if newValue, exists := c.Get("new_value"); exists {
		audit.NewValue = fmt.Sprintf("%v", newValue)
	}

	// 写入审计日志
	writeAdminAuditLog(audit)
}

// writeAdminAuditLog 写入管理员审计日志
func writeAdminAuditLog(audit AdminAuditLog) {
	auditMutex.Lock()
	defer auditMutex.Unlock()

	if adminAuditFile != nil {
		line := fmt.Sprintf(
			"[%s] admin_id=%d admin=%s op=%s target=%s/%s ip=%s totp=%v\n",
			audit.CreatedAt.Format(time.RFC3339),
			audit.AdminID,
			audit.AdminName,
			audit.Operation,
			audit.TargetType,
			audit.TargetID,
			audit.IP,
			audit.TOTPVefified,
		)
		adminAuditFile.WriteString(line)
	}

	// 同时写入数据库（需要在 New-API 中实现）
	// database.DB.Create(&audit)
}

// AdminOnly 仅管理员可访问
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get("admin_user"); !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "admin required",
			})
			return
		}
		c.Next()
	}
}

// SuperAdminOnly 仅超级管理员可访问
func SuperAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		adminUser, exists := c.Get("admin_user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "admin required",
			})
			return
		}

		admin := adminUser.(*AdminUser)
		if admin.Role != RoleSuperAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "super admin required",
			})
			return
		}

		c.Next()
	}
}

// IPWhitelist IP 白名单中间件
func IPWhitelist(allowedIPs []string) gin.HandlerFunc {
	allowedSet := make(map[string]bool)
	for _, ip := range allowedIPs {
		allowedSet[ip] = true
	}

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// 检查是否在白名单中
		if !allowedSet[clientIP] {
			// 检查是否是 CIDR 格式
			allowed := false
			for _, allowedIP := range allowedIPs {
				if strings.Contains(allowedIP, "/") {
					// CIDR 检查需要额外实现
					// 简化版：直接字符串匹配
					if strings.HasPrefix(clientIP, strings.Split(allowedIP, "/")[0][:strings.LastIndex(strings.Split(allowedIP, "/")[0], ".")]) {
						allowed = true
						break
					}
				}
			}

			if !allowed {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "IP not in whitelist",
				})
				return
			}
		}

		c.Next()
	}
}

// RateLimitByAdmin 管理员级别限流
func RateLimitByAdmin(ratePerMinute int) gin.HandlerFunc {
	// 简化实现，生产环境应使用 Redis
	type limiter struct {
		count   int
		resetAt time.Time
	}

	limiters := make(map[uint]*limiter)
	var mu sync.Mutex

	return func(c *gin.Context) {
		adminUser, exists := c.Get("admin_user")
		if !exists {
			c.Next()
			return
		}

		admin := adminUser.(*AdminUser)
		mu.Lock()
		defer mu.Unlock()

		now := time.Now()
		if l, exists := limiters[admin.ID]; exists {
			if now.After(l.resetAt) {
				l.count = 1
				l.resetAt = now.Add(time.Minute)
			} else {
				l.count++
				if l.count > ratePerMinute {
					c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
						"error":       "rate limit exceeded",
						"retry_after": l.resetAt.Sub(now).Seconds(),
					})
					return
				}
			}
		} else {
			limiters[admin.ID] = &limiter{
				count:   1,
				resetAt: now.Add(time.Minute),
			}
		}

		c.Next()
	}
}

// AuditLogMiddleware 审计日志中间件（记录所有管理操作）
func AuditLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 仅记录管理端操作
		if !strings.HasPrefix(c.Request.URL.Path, "/admin") {
			c.Next()
			return
		}

		// 记录请求开始时间
		start := time.Now()

		// 处理请求
		c.Next()

		// 记录审计日志
		duration := time.Since(start)
		log.Printf("[ADMIN_AUDIT] method=%s path=%s status=%d duration=%dms ip=%s",
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration.Milliseconds(),
			c.ClientIP(),
		)
	}
}
