//go:build tt
// +build tt

package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSecurityProxy_LevelMeta(t *testing.T) {
	// 设置日志级别为META
	os.Setenv("TT_LOG_LEVEL", "1")

	router := gin.New()
	router.Use(middleware.SecurityProxy())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer tk-test123")
	req.Header.Set("X-Request-ID", "req-123")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSecurityProxy_LevelSilent(t *testing.T) {
	os.Setenv("TT_LOG_LEVEL", "0")

	router := gin.New()
	router.Use(middleware.SecurityProxy())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSecurityProxy_RequestBodyLimit(t *testing.T) {
	router := gin.New()
	router.Use(middleware.SecurityProxy())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Body = nil // 由MaxBytesReader处理

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 由于请求体大小限制，应该能正常处理（中间件层面不拦截）
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestClaudeCodeCompatibility(t *testing.T) {
	router := gin.New()
	router.Use(middleware.ClaudeCodeCompatibility())
	router.POST("/test", func(c *gin.Context) {
		// 检查中间件设置的信息
		clientType, exists := c.Get("client_type")
		if !exists {
			t.Error("client_type not set")
		}
		if clientType != "claude_code" {
			t.Errorf("Expected client_type 'claude_code', got '%v'", clientType)
		}

		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("User-Agent", "claude-code/1.0")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("X-Claude-Code-Session-Id", "session-123")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequestIDMiddleware())
	router.GET("/test", func(c *gin.Context) {
		requestID, exists := c.Get("request_id")
		if !exists {
			t.Error("request_id not set")
		}
		if requestID == "" {
			t.Error("request_id is empty")
		}
		c.JSON(http.StatusOK, gin.H{"request_id": requestID})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// 检查响应头中的X-Request-ID
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header not set")
	}
}

func TestSetLogLevel(t *testing.T) {
	// 测试日志级别设置
	tests := []struct {
		name     string
		level    middleware.LogLevel
		expected middleware.LogLevel
	}{
		{"Silent", middleware.LevelSilent, middleware.LevelSilent},
		{"Meta", middleware.LevelMeta, middleware.LevelMeta},
		{"Debug", middleware.LevelDebug, middleware.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := middleware.SetLogLevel(tt.level, "test")
			if err != nil {
				t.Errorf("SetLogLevel failed: %v", err)
			}

			current := middleware.GetLogLevel()
			if current != tt.expected {
				t.Errorf("Expected level %d, got %d", tt.expected, current)
			}
		})
	}
}

func TestHashUserID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"12345", "****"},
		{"abc", "****"},
		{"1234567890", "1234****7890"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := middleware.HashUserID(tt.input)
			if result != tt.expected {
				t.Errorf("HashUserID(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}
