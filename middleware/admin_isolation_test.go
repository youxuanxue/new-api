//go:build tt
// +build tt

package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAdminIsolation_RouteIsolation(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// 模拟已认证的管理员
		c.Set("admin_user", &middleware.AdminUser{
			ID:       1,
			Username: "test_admin",
			Role:     middleware.RoleOperator,
			IsActive: true,
		})
		c.Next()
	})
	router.Use(middleware.AdminIsolation())

	router.GET("/admin/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	// 测试管理端路由
	t.Run("Admin route allowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 未定义的管理操作默认仅 super_admin 允许。
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}
	})

	// 测试非管理端路由（应该正常通过）
	t.Run("Non-admin route allowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

func TestAdminIsolation_Unauthorized(t *testing.T) {
	router := gin.New()
	router.Use(middleware.AdminIsolation())
	router.GET("/admin/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/admin/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 没有认证，应该返回401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestAdminIsolation_DisabledAccount(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("admin_user", &middleware.AdminUser{
			ID:       1,
			Username: "disabled_admin",
			Role:     middleware.RoleOperator,
			IsActive: false, // 已禁用
		})
		c.Next()
	})
	router.Use(middleware.AdminIsolation())
	router.GET("/admin/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/admin/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 禁用账号，应该返回403
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestAdminIsolation_RolePermissions(t *testing.T) {
	tests := []struct {
		name       string
		role       middleware.AdminRole
		method     string
		path       string
		expected   int
	}{
		{"SuperAdmin can view finance", middleware.RoleSuperAdmin, "GET", "/admin/finance/overview", http.StatusOK},
		{"Operator can manage users", middleware.RoleOperator, "GET", "/admin/users", http.StatusOK},
		{"Viewer can only view", middleware.RoleViewer, "GET", "/admin/users", http.StatusOK},
		{"Viewer cannot create pricing", middleware.RoleViewer, "POST", "/admin/pricing", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("admin_user", &middleware.AdminUser{
					ID:       1,
					Username: "test_admin",
					Role:     tt.role,
					IsActive: true,
				})
				c.Next()
			})
			router.Use(middleware.AdminIsolation())

			router.GET("/admin/users", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "ok"})
			})
			router.POST("/admin/pricing", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "ok"})
			})
			router.GET("/admin/finance/overview", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "ok"})
			})

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestAdminIsolation_CriticalOperationsRequireTOTP(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("admin_user", &middleware.AdminUser{
			ID:         1,
			Username:   "root",
			Role:       middleware.RoleSuperAdmin,
			TOTPSecret: "JBSWY3DPEHPK3PXP",
			IsActive:   true,
		})
		c.Next()
	})
	router.Use(middleware.AdminIsolation())
	router.POST("/admin/users/:id/adjust-balance", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
	router.POST("/admin/users/:id/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
	router.POST("/admin/users/:id/admin-role", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []string{
		"/admin/users/123/adjust-balance",
		"/admin/users/987/status",
		"/admin/users/42/admin-role",
	}
	for _, path := range tests {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{}`))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for %s without TOTP, got %d", path, w.Code)
		}
	}
}

func TestIPWhitelist(t *testing.T) {
	allowedIPs := []string{"192.168.1.1", "10.0.0.0/8"}

	router := gin.New()
	router.Use(middleware.IPWhitelist(allowedIPs))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []struct {
		name     string
		clientIP string
		expected int
	}{
		{"Allowed IP", "192.168.1.1", http.StatusOK},
		{"Blocked IP", "8.8.8.8", http.StatusForbidden},
		{"CIDR match", "10.0.0.100", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.clientIP + ":12345"
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestCheckPermission(t *testing.T) {
	tests := []struct {
		userRole    middleware.AdminRole
		operation   string
		shouldAllow bool
	}{
		// Super admin can do everything
		{middleware.RoleSuperAdmin, "POST /admin/pricing", true},
		{middleware.RoleSuperAdmin, "GET /admin/finance/overview", true},
		{middleware.RoleSuperAdmin, "GET /admin/audit", true},
		{middleware.RoleSuperAdmin, "POST /admin/users/admin-role", true},

		// Operator can read + operate, but not finance/audit/pricing-write
		{middleware.RoleOperator, "GET /admin/users", true},
		{middleware.RoleOperator, "PUT /admin/users", true},
		{middleware.RoleOperator, "POST /admin/channels", true},
		{middleware.RoleOperator, "POST /admin/channels/test", true},
		{middleware.RoleOperator, "POST /admin/pool/accounts", true},
		{middleware.RoleOperator, "GET /admin/settings", true},
		{middleware.RoleOperator, "POST /admin/webhooks", true},
		{middleware.RoleOperator, "POST /admin/pricing", false},
		{middleware.RoleOperator, "GET /admin/finance/overview", false},
		{middleware.RoleOperator, "GET /admin/audit", false},
		{middleware.RoleOperator, "DELETE /admin/channels", false},
		{middleware.RoleOperator, "POST /admin/users/admin-role", false},

		// Viewer can only read
		{middleware.RoleViewer, "GET /admin/users", true},
		{middleware.RoleViewer, "GET /admin/dashboard", true},
		{middleware.RoleViewer, "GET /admin/channels", true},
		{middleware.RoleViewer, "GET /admin/pool", true},
		{middleware.RoleViewer, "GET /admin/pricing", true},
		{middleware.RoleViewer, "GET /admin/plans", true},
		{middleware.RoleViewer, "GET /admin/webhooks", true},
		{middleware.RoleViewer, "PUT /admin/users", false},
		{middleware.RoleViewer, "POST /admin/channels", false},
		{middleware.RoleViewer, "GET /admin/settings", false},
		{middleware.RoleViewer, "GET /admin/finance/overview", false},

		// Unknown operations default to super_admin only
		{middleware.RoleSuperAdmin, "POST /admin/unknown", true},
		{middleware.RoleOperator, "POST /admin/unknown", false},
		{middleware.RoleViewer, "POST /admin/unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.userRole)+"_"+tt.operation, func(t *testing.T) {
			allowed := middleware.CheckPermission(tt.userRole, tt.operation)
			if allowed != tt.shouldAllow {
				t.Errorf("CheckPermission(%s, %s) = %v, expected %v",
					tt.userRole, tt.operation, allowed, tt.shouldAllow)
			}
		})
	}
}
