// Package middleware_test 测试管理后台隔离中间件
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAdminIsolation_RouteIsolation(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// 模拟已认证的管理员
		c.Set("admin_user", &ttmodel.Admin{
			Id:       1,
			Username: "test_admin",
			Role:     ttmodel.RoleOperator,
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

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
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
		c.Set("admin_user", &ttmodel.Admin{
			Id:       1,
			Username: "disabled_admin",
			Role:     ttmodel.RoleOperator,
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
		role       ttmodel.AdminRole
		path       string
		expected   int
	}{
		{"SuperAdmin can do anything", ttmodel.RoleSuperAdmin, "/admin/pricing", http.StatusOK},
		{"Operator can manage users", ttmodel.RoleOperator, "/admin/users", http.StatusOK},
		{"Viewer can only view", ttmodel.RoleViewer, "/admin/users", http.StatusOK},
		{"Viewer cannot create pricing", ttmodel.RoleViewer, "/admin/pricing", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("admin_user", &ttmodel.Admin{
					Id:       1,
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

			req := httptest.NewRequest("POST", tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, w.Code)
			}
		})
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
		userRole   ttmodel.AdminRole
		operation  string
		shouldAllow bool
	}{
		{ttmodel.RoleSuperAdmin, "POST /admin/pricing", true},
		{ttmodel.RoleOperator, "GET /admin/users", true},
		{ttmodel.RoleOperator, "POST /admin/pricing", false},
		{ttmodel.RoleViewer, "GET /admin/users", true},
		{ttmodel.RoleViewer, "PUT /admin/users", false},
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
