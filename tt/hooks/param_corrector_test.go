package hooks

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
)

func testGinContext(t *testing.T, userAgent string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	c.Request = req
	return c
}

func TestUS140_ModelAliasClaudeSonnet(t *testing.T) {
	pc := NewTTParamCorrector()
	req := &dto.GeneralOpenAIRequest{Model: "claude-sonnet"}
	pc.CorrectRequest(testGinContext(t, ""), req)
	if req.Model != "claude-sonnet-4-6" {
		t.Fatalf("alias: want model claude-sonnet-4-6, got %q", req.Model)
	}
}

func TestUS140_DeprecatedClaude2Forwarded(t *testing.T) {
	pc := NewTTParamCorrector()
	req := &dto.GeneralOpenAIRequest{Model: "claude-2"}
	pc.CorrectRequest(testGinContext(t, ""), req)
	if req.Model != "claude-sonnet-4-6" {
		t.Fatalf("deprecated: want claude-sonnet-4-6, got %q", req.Model)
	}
}

func TestUS140_MaxTokensCappedForClaudeSonnet(t *testing.T) {
	pc := NewTTParamCorrector()
	req := &dto.GeneralOpenAIRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: common.GetPointer(uint(50000)),
	}
	pc.CorrectRequest(testGinContext(t, ""), req)
	if req.MaxTokens == nil || *req.MaxTokens != 16000 {
		t.Fatalf("max_tokens: want 16000, got %v", req.MaxTokens)
	}
}

func TestUS140_NoMaxTokenChangeWhenWithinLimit(t *testing.T) {
	pc := NewTTParamCorrector()
	req := &dto.GeneralOpenAIRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: common.GetPointer(uint(8000)),
	}
	pc.CorrectRequest(testGinContext(t, ""), req)
	if req.MaxTokens == nil || *req.MaxTokens != 8000 {
		t.Fatalf("max_tokens: want 8000 unchanged, got %v", req.MaxTokens)
	}
}

func TestUS140_StreamAutoCompleteForClaudeCodeUA(t *testing.T) {
	pc := NewTTParamCorrector()
	req := &dto.GeneralOpenAIRequest{Model: "gpt-4o"}
	pc.CorrectRequest(testGinContext(t, "Claude Code/1.0"), req)
	if req.Stream == nil || !*req.Stream {
		t.Fatalf("stream: want auto true for Claude Code UA, got stream=%v", req.Stream)
	}
}

func TestUS141_Gpt4oAliasToCanonicalOpenAIName(t *testing.T) {
	pc := NewTTParamCorrector()
	req := &dto.GeneralOpenAIRequest{Model: "gpt4o"}
	pc.CorrectRequest(testGinContext(t, ""), req)
	if req.Model != "gpt-4o" {
		t.Fatalf("gpt4o alias: want gpt-4o, got %q", req.Model)
	}
}

func TestUS141_CanonicalGpt4oUnchangedByAliasTable(t *testing.T) {
	pc := NewTTParamCorrector()
	req := &dto.GeneralOpenAIRequest{Model: "gpt-4o"}
	pc.CorrectRequest(testGinContext(t, ""), req)
	if req.Model != "gpt-4o" {
		t.Fatalf("canonical name must stay gpt-4o, got %q", req.Model)
	}
}

func TestUS140_RelaySuccessAddsModelAliasHeader(t *testing.T) {
	pc := NewTTParamCorrector()
	h := &TTRelayHooks{paramCorrector: pc}
	c := testGinContext(t, "")
	req := &dto.GeneralOpenAIRequest{Model: "claude-sonnet"}
	pc.CorrectRequest(c, req)
	res := h.OnRelaySuccess(&HookContext{GinContext: c, Request: req})
	if res == nil || !res.Modified {
		t.Fatalf("expected relay hook to emit correction headers, got %+v", res)
	}
	want := "claude-sonnet->claude-sonnet-4-6"
	if got := res.Headers["X-Param-Correction-Model-Alias"]; got != want {
		t.Fatalf("X-Param-Correction-Model-Alias: want %q, got %q", want, got)
	}
}

func TestUS140_RelaySuccessAddsDeprecatedHeader(t *testing.T) {
	pc := NewTTParamCorrector()
	h := &TTRelayHooks{paramCorrector: pc}
	c := testGinContext(t, "")
	req := &dto.GeneralOpenAIRequest{Model: "claude-2"}
	pc.CorrectRequest(c, req)
	res := h.OnRelaySuccess(&HookContext{GinContext: c, Request: req})
	if res == nil || !res.Modified {
		t.Fatalf("expected headers, got %+v", res)
	}
	want := "claude-2->claude-sonnet-4-6"
	if got := res.Headers["X-Param-Correction-Deprecated"]; got != want {
		t.Fatalf("deprecated header: want %q, got %q", want, got)
	}
}

func TestUS140_RelaySuccessAddsMaxTokensHeaderWhenCapped(t *testing.T) {
	pc := NewTTParamCorrector()
	h := &TTRelayHooks{paramCorrector: pc}
	c := testGinContext(t, "")
	req := &dto.GeneralOpenAIRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: common.GetPointer(uint(50000)),
	}
	pc.CorrectRequest(c, req)
	res := h.OnRelaySuccess(&HookContext{GinContext: c, Request: req})
	if res == nil || !res.Modified {
		t.Fatalf("expected headers, got %+v", res)
	}
	if got := res.Headers["X-Param-Correction-Max-Tokens"]; got != "50000->16000" {
		t.Fatalf("max tokens header: want 50000->16000, got %q", got)
	}
}
