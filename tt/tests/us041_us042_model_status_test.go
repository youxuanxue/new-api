package tests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	ttcontroller "github.com/QuantumNous/new-api/tt/controller"
	ttservice "github.com/QuantumNous/new-api/tt/service"
	"github.com/gin-gonic/gin"
)

func newTTContext(t *testing.T, method string, target string, body []byte, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	if userID > 0 {
		ctx.Set("id", userID)
	}
	return ctx, recorder
}

func TestUS041_VerifyModelMissingConfig(t *testing.T) {
	t.Setenv("TT_VERIFY_API_URL", "")
	t.Setenv("TT_VERIFY_API_KEY", "")
	t.Setenv("ANTHROPIC_BASE_URL", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("OPENAI_API_KEY", "")

	result, err := ttservice.VerifyModelAuthenticity("claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("verify should return structured result, got error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed status when verify config is missing, got %s", result.Status)
	}
	if !strings.Contains(result.Message, "missing verify API config") {
		t.Fatalf("expected missing config message, got %q", result.Message)
	}
}

func TestUS041_VerifyModelThinkingDetected(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Keep latency above suspicious threshold in verifier.
		time.Sleep(60 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "VERIFICATION_OK",
					"thinking": "internal reasoning trace"
				}
			}]
		}`))
	}))
	defer upstream.Close()

	t.Setenv("TT_VERIFY_API_URL", upstream.URL)
	t.Setenv("TT_VERIFY_API_KEY", "test-key")

	result, err := ttservice.VerifyModelAuthenticity("claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("verify should not return error: %v", err)
	}
	if result.Status != "verified" {
		t.Fatalf("expected verified status, got %s (message=%q)", result.Status, result.Message)
	}
	if !result.ThinkingDetected {
		t.Fatal("expected thinking_detected=true for claude model with thinking field")
	}
	if result.ResponseTime < 50 {
		t.Fatalf("expected response_time_ms >= 50, got %d", result.ResponseTime)
	}
}

func TestUS041_VerifyModelInvalidResponseStructure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer upstream.Close()

	t.Setenv("TT_VERIFY_API_URL", upstream.URL)
	t.Setenv("TT_VERIFY_API_KEY", "test-key")

	result, err := ttservice.VerifyModelAuthenticity("claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("verify should return structured failure result, got error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed status for invalid response structure, got %s", result.Status)
	}
	if !strings.Contains(result.Message, "invalid response structure") {
		t.Fatalf("expected invalid structure message, got %q", result.Message)
	}
}

func TestUS042_ViewServiceStatus(t *testing.T) {
	ctx, recorder := newTTContext(t, http.MethodGet, "/tt/status", nil, 0)
	ttcontroller.GetServiceStatus(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}

	var resp struct {
		Services map[string]ttcontroller.ServiceInfo `json:"services"`
	}
	if err := common.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode service status response: %v", err)
	}
	if len(resp.Services) == 0 {
		t.Fatal("expected non-empty services map")
	}

	claude, ok := resp.Services["claude-sonnet-4-6"]
	if !ok {
		t.Fatal("expected claude-sonnet-4-6 service in status response")
	}
	if claude.Status != "operational" {
		t.Fatalf("expected claude-sonnet-4-6 status operational, got %s", claude.Status)
	}
	if claude.Uptime <= 0 || claude.Uptime > 1 {
		t.Fatalf("expected uptime in (0,1], got %f", claude.Uptime)
	}
	if claude.Latency <= 0 {
		t.Fatalf("expected positive latency, got %d", claude.Latency)
	}
}

func TestUS042_PublicStatusMatchesServiceStatusShape(t *testing.T) {
	ctx, recorder := newTTContext(t, http.MethodGet, "/tt/public/status", nil, 0)
	ttcontroller.GetPublicStatus(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}

	var resp map[string]any
	if err := common.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode public status response: %v", err)
	}
	servicesRaw, ok := resp["services"]
	if !ok {
		t.Fatalf("expected services field in public status response, got %v", resp)
	}
	services, ok := servicesRaw.(map[string]any)
	if !ok || len(services) == 0 {
		t.Fatalf("expected non-empty services object, got %T %+v", servicesRaw, servicesRaw)
	}
}
