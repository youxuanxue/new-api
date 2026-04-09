//go:build tt
// +build tt

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
)

func TestUS130_HashUserIDRedacts(t *testing.T) {
	if got := middleware.HashUserID("short"); got != "****" {
		t.Fatalf("short id should redact to ****, got %q", got)
	}
	got := middleware.HashUserID("user12345678")
	if len(got) < 8 || got == "user12345678" {
		t.Fatalf("expected masked user id, got %q", got)
	}
}

func TestUS130_RequestMetaJSONExcludesRequestBody(t *testing.T) {
	meta := middleware.RequestMeta{
		RequestID:  "rid",
		Method:     "POST",
		Path:       "/v1/chat/completions",
		Model:      "gpt-4",
		UserID:     middleware.HashUserID("12345"),
		StatusCode: 200,
	}
	raw, err := common.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"request_body", "body", "messages", "content"} {
		if _, ok := m[key]; ok {
			t.Fatalf("RequestMeta JSON must not include payload field %q (got keys: %v)", key, m)
		}
	}
}

func TestUS132_AdminIsolation_CriticalOpRequiresTOTPHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "tt", AccountName: "admin"})
	if err != nil {
		t.Fatalf("totp.Generate: %v", err)
	}
	admin := &middleware.AdminUser{
		ID:         1,
		Username:   "sa",
		Role:       middleware.RoleSuperAdmin,
		TOTPSecret: key.Secret(),
		IsActive:   true,
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("admin_user", admin)
		c.Next()
	})
	r.Use(middleware.AdminIsolation())
	r.POST("/admin/users/:id/adjust-balance", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/users/9/adjust-balance", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without TOTP header, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body json: %v", err)
	}
	if body["error"] != "2FA required" {
		t.Fatalf("expected 2FA required error, got %#v", body)
	}
}

func TestUS132_AdminIsolation_ValidTOTPAllowsCriticalOp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "tt", AccountName: "admin2"})
	if err != nil {
		t.Fatalf("totp.Generate: %v", err)
	}
	secret := key.Secret()
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	admin := &middleware.AdminUser{
		ID:         2,
		Username:   "sa2",
		Role:       middleware.RoleSuperAdmin,
		TOTPSecret: secret,
		IsActive:   true,
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("admin_user", admin)
		c.Next()
	})
	r.Use(middleware.AdminIsolation())
	r.POST("/admin/users/:id/adjust-balance", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/users/9/adjust-balance", nil)
	req.Header.Set("X-TOTP-Code", code)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid TOTP, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUS132_AdminIsolation_InvalidTOTPRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "tt", AccountName: "admin3"})
	if err != nil {
		t.Fatalf("totp.Generate: %v", err)
	}
	admin := &middleware.AdminUser{
		ID:         3,
		Username:   "sa3",
		Role:       middleware.RoleSuperAdmin,
		TOTPSecret: key.Secret(),
		IsActive:   true,
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("admin_user", admin)
		c.Next()
	})
	r.Use(middleware.AdminIsolation())
	r.POST("/admin/users/:id/adjust-balance", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/users/9/adjust-balance", nil)
	req.Header.Set("X-TOTP-Code", "000000")
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for bad TOTP, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body json: %v", err)
	}
	if body["error"] != "invalid TOTP code" {
		t.Fatalf("expected invalid TOTP code, got %#v", body)
	}
}

func TestUS133_IPWhitelistAllowsExactMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.IPWhitelist([]string{"127.0.0.1"}))
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "127.0.0.1:5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for whitelisted IP, got %d", w.Code)
	}
}

func TestUS133_IPWhitelistBlocksUnknownClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.IPWhitelist([]string{"127.0.0.1"}))
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "192.168.1.50:5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-whitelisted IP, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if body["error"] != "IP not in whitelist" {
		t.Fatalf("expected whitelist error, got %#v", body)
	}
}

func TestUS133_IPWhitelistAllowsCIDR(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.IPWhitelist([]string{"10.0.0.0/8"}))
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "10.20.30.40:5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for IP in CIDR, got %d", w.Code)
	}
}

func TestUS134_RateLimitByAdminReturns429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	admin := &middleware.AdminUser{ID: 99, Username: "rl", Role: middleware.RoleViewer, IsActive: true}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("admin_user", admin)
		c.Next()
	})
	r.Use(middleware.RateLimitByAdmin(2))
	r.GET("/admin/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	var lastCode int
	var lastBody string
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/admin/ping", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		lastCode = w.Code
		lastBody = w.Body.String()
		if i < 2 && w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected third request to be 429, got %d body=%s", lastCode, lastBody)
	}
	var rl map[string]any
	if err := json.Unmarshal([]byte(lastBody), &rl); err != nil {
		t.Fatalf("429 body json: %v", err)
	}
	if rl["error"] != "rate limit exceeded" {
		t.Fatalf("expected rate limit error, got %#v", rl)
	}
}

func TestUS135_SameIPReferralCooldownBlocksSecondInvitee(t *testing.T) {
	inviter := &ttmodel.User{
		Username: "inviter-us135-ip",
		Email:    "inviter-us135-ip@example.com",
		AffCode:  "INVITE-US135-IP",
		Status:   1,
	}
	if err := testDB.Create(inviter).Error; err != nil {
		t.Fatalf("create inviter: %v", err)
	}
	invitee1 := &ttmodel.User{
		Username: "invitee-us135-a",
		Email:    "invitee-us135-a@example.com",
		AffCode:  nextAffCode("US135A"),
		Status:   1,
	}
	invitee2 := &ttmodel.User{
		Username: "invitee-us135-b",
		Email:    "invitee-us135-b@example.com",
		AffCode:  nextAffCode("US135B"),
		Status:   1,
	}
	if err := testDB.Create(invitee1).Error; err != nil {
		t.Fatalf("create invitee1: %v", err)
	}
	if err := testDB.Create(invitee2).Error; err != nil {
		t.Fatalf("create invitee2: %v", err)
	}

	const sharedIP = "203.0.113.50"
	if _, err := ttmodel.ApplyReferralCode(int(invitee1.Id), "INVITE-US135-IP", sharedIP); err != nil {
		t.Fatalf("first referral on IP should succeed: %v", err)
	}
	_, err := ttmodel.ApplyReferralCode(int(invitee2.Id), "INVITE-US135-IP", sharedIP)
	if err == nil {
		t.Fatal("expected second referral from same IP within cooldown to fail")
	}
	if err.Error() != "同一IP在24小时内只能使用一次邀请码" {
		t.Fatalf("unexpected error: %v", err)
	}
}
