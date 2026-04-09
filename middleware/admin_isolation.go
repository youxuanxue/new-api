//go:build tt
// +build tt

// Package middleware 提供TT核心中间件
// admin_isolation.go - 管理后台隔离中间件，实现路由隔离、敏感操作审计、关键操作二次验证
package middleware

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	ttmodel "github.com/QuantumNous/new-api/model"
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

// sensitiveOperations are logged to the audit trail (file + DB).
var sensitiveOperations = map[string]bool{
	"PUT /admin/users":                 true,
	"DELETE /admin/users":              true,
	"POST /admin/users/adjust-balance": true,
	"POST /admin/users/status":         true,
	"POST /admin/users/admin-role":     true,
	"POST /admin/channels":             true,
	"PUT /admin/channels":              true,
	"DELETE /admin/channels":           true,
	"POST /admin/pricing":              true,
	"PUT /admin/pricing":               true,
	"POST /admin/plans":                true,
	"PUT /admin/plans":                 true,
	"POST /admin/pool/accounts":        true,
	"DELETE /admin/pool/accounts":      true,
	"PUT /admin/settings":              true,
	"POST /admin/webhooks":             true,
	"PUT /admin/webhooks":              true,
	"DELETE /admin/webhooks":           true,
}

// criticalOperations require a valid X-TOTP-Code header (2FA).
var criticalOperations = map[string]bool{
	"POST /admin/users/adjust-balance": true,
	"POST /admin/users/status":         true,
	"POST /admin/users/admin-role":     true,
	"POST /admin/pricing":              true,
	"PUT /admin/pricing":               true,
	"DELETE /admin/channels":           true,
	"DELETE /admin/pool/accounts":      true,
	"PUT /admin/settings":              true,
}

// operationPermissions maps "METHOD /normalized/path" to the minimum
// AdminRole required.  Any route NOT listed here defaults to super_admin
// (deny-by-default).  The path is post-normalizeAdminPath (ID segments stripped).
var operationPermissions = map[string]AdminRole{
	// Dashboard
	"GET /admin/dashboard": RoleViewer,

	// Users
	"GET /admin/users":                 RoleViewer,
	"PUT /admin/users":                 RoleOperator,
	"DELETE /admin/users":              RoleSuperAdmin,
	"POST /admin/users/adjust-balance": RoleSuperAdmin,
	"POST /admin/users/status":         RoleOperator,
	"POST /admin/users/admin-role":     RoleSuperAdmin,
	"GET /admin/users/admin-roles":     RoleSuperAdmin,

	// Channels
	"GET /admin/channels":    RoleViewer,
	"POST /admin/channels":   RoleOperator,
	"PUT /admin/channels":    RoleOperator,
	"DELETE /admin/channels": RoleSuperAdmin,
	"POST /admin/channels/test": RoleOperator,

	// Pool
	"GET /admin/pool":                    RoleViewer,
	"GET /admin/pool/accounts":           RoleViewer,
	"POST /admin/pool/accounts":          RoleOperator,
	"DELETE /admin/pool/accounts":        RoleSuperAdmin,
	"POST /admin/pool/accounts/refresh":  RoleOperator,

	// Pricing
	"GET /admin/pricing":  RoleViewer,
	"POST /admin/pricing": RoleSuperAdmin,
	"PUT /admin/pricing":  RoleSuperAdmin,

	// Plans
	"GET /admin/plans":  RoleViewer,
	"POST /admin/plans": RoleSuperAdmin,
	"PUT /admin/plans":  RoleSuperAdmin,

	// Finance
	"GET /admin/finance/overview": RoleSuperAdmin,
	"GET /admin/finance/revenue":  RoleSuperAdmin,
	"GET /admin/finance/costs":    RoleSuperAdmin,
	"GET /admin/finance/payments": RoleSuperAdmin,

	// Audit
	"GET /admin/audit": RoleSuperAdmin,

	// Settings
	"GET /admin/settings": RoleOperator,
	"PUT /admin/settings": RoleSuperAdmin,

	// Webhooks
	"GET /admin/webhooks":     RoleViewer,
	"POST /admin/webhooks":    RoleOperator,
	"PUT /admin/webhooks":     RoleOperator,
	"DELETE /admin/webhooks":  RoleSuperAdmin,
	"POST /admin/webhooks/test": RoleOperator,
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
	TOTPVerified bool     `json:"totp_verified"`
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

		// 2. 检查操作权限（normalize path to strip param segments like :id)
		opKey := c.Request.Method + " " + normalizeAdminPath(path)
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
			audit := buildAdminAudit(c, admin, opKey)
			go writeAdminAuditLog(audit)
		}

		c.Next()
	}
}

// normalizeAdminPath strips numeric/UUID ID segments from the path so that
// /admin/users/123 becomes /admin/users and /admin/channels/42/test becomes
// /admin/channels/test. This allows exact-match permission lookups.
func normalizeAdminPath(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	var normalized []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		if isIDSegment(p) {
			continue
		}
		normalized = append(normalized, p)
	}
	return "/" + strings.Join(normalized, "/")
}

func isIDSegment(s string) bool {
	if s == "" {
		return false
	}
	// UUID-like IDs: 8-4-4-4-12 hex groups
	if len(s) == 36 && strings.Count(s, "-") == 4 {
		for _, c := range s {
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-' {
				continue
			}
			return false
		}
		return true
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// CheckPermission reports whether role may perform operation (exported for tests).
func CheckPermission(role AdminRole, operation string) bool {
	return checkPermission(role, operation)
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

// buildAdminAudit captures audit fields from request context synchronously.
func buildAdminAudit(c *gin.Context, admin *AdminUser, operation string) AdminAuditLog {
	audit := AdminAuditLog{
		AdminID:      admin.ID,
		AdminName:    admin.Username,
		Operation:    operation,
		TargetID:     c.Param("id"),
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		TOTPVerified: c.GetBool("totp_verified"),
		CreatedAt:    time.Now(),
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

	return audit
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
			audit.TOTPVerified,
		)
		adminAuditFile.WriteString(line)
	}

	// 同步持久化到 DB，确保容器重启后可追溯。
	if ttmodel.DB != nil {
		dbAudit := ttmodel.AdminAuditLog{
			AdminId:      audit.AdminID,
			AdminName:    audit.AdminName,
			Operation:    audit.Operation,
			TargetId:     audit.TargetID,
			TargetType:   audit.TargetType,
			OldValue:     audit.OldValue,
			NewValue:     audit.NewValue,
			IP:           audit.IP,
			UserAgent:    audit.UserAgent,
			TOTPVerified: audit.TOTPVerified,
			CreatedAt:    audit.CreatedAt,
		}
		if err := ttmodel.DB.Create(&dbAudit).Error; err != nil {
			log.Printf("[WARN] failed to persist admin audit log: %v", err)
		}
	}
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
	exactIPs := make(map[string]bool)
	var cidrNets []*net.IPNet

	for _, entry := range allowedIPs {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err != nil {
				log.Printf("[WARN] invalid CIDR in IP whitelist: %s: %v", entry, err)
				continue
			}
			cidrNets = append(cidrNets, ipNet)
		} else {
			exactIPs[entry] = true
		}
	}

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		if exactIPs[clientIP] {
			c.Next()
			return
		}

		ip := net.ParseIP(clientIP)
		if ip != nil {
			for _, cidr := range cidrNets {
				if cidr.Contains(ip) {
					c.Next()
					return
				}
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "IP not in whitelist",
		})
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
