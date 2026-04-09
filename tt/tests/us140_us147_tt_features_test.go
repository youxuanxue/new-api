package tests

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	ttcontroller "github.com/QuantumNous/new-api/tt/controller"
)

func TestUS142_TTRouterHasNoSemanticCacheRoutes(t *testing.T) {
	// Repo-root relative: tests run with cwd = new-api/tt/tests
	routerFile := filepath.Join("..", "..", "router", "tt-router.go")
	b, err := os.ReadFile(routerFile)
	if err != nil {
		t.Fatalf("read tt-router: %v", err)
	}
	s := string(b)
	if strings.Contains(s, `"/tt/cache`) || strings.Contains(s, `'/tt/cache`) {
		t.Fatal("US-140 AC-006: semantic cache routes must not be registered until feature ships")
	}
}

func TestUS143_GetCostReport_Unauthorized(t *testing.T) {
	ctx, rec := newTTContext(t, http.MethodGet, "/tt/reports/cost", nil, 0)
	ttcontroller.GetCostReport(ctx)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 unauthorized, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUS143_GetCostReport_OKShape(t *testing.T) {
	user := createBillingTestUser(t, "US143", 1000)
	ctx, rec := newTTContext(t, http.MethodGet, "/tt/reports/cost?start_date=2026-01-01&end_date=2026-01-31", nil, user.Id)
	ttcontroller.GetCostReport(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ReportType    string `json:"report_type"`
		TotalCost     string `json:"total_cost"`
		TotalRequests int64  `json:"total_requests"`
	}
	if err := common.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ReportType == "" || resp.TotalCost == "" {
		t.Fatalf("expected report_type and total_cost, got %+v", resp)
	}
}

func TestUS143_ModelBreakdown_Unauthorized(t *testing.T) {
	ctx, rec := newTTContext(t, http.MethodGet, "/tt/reports/breakdown/models?start_date=2026-01-01&end_date=2026-01-31", nil, 0)
	ttcontroller.GetModelCostBreakdown(ctx)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestUS143_ModelBreakdown_OKReturnsArray(t *testing.T) {
	user := createBillingTestUser(t, "US143B", 1000)
	ctx, rec := newTTContext(t, http.MethodGet, "/tt/reports/breakdown/models?start_date=2026-01-01&end_date=2026-01-31", nil, user.Id)
	ttcontroller.GetModelCostBreakdown(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var wrap struct {
		Data []map[string]any `json:"data"`
	}
	if err := common.Unmarshal(rec.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(wrap.Data) < 1 {
		t.Fatalf("expected non-empty model breakdown list, got %d rows", len(wrap.Data))
	}
}

func TestUS145_PlaygroundRun_RejectsMoreThanFourModels(t *testing.T) {
	user := createBillingTestUser(t, "US145", 1000000)
	body, err := common.Marshal(map[string]any{
		"models": []string{"a", "b", "c", "d", "e"},
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, rec := newTTContext(t, http.MethodPost, "/tt/playground/run", body, user.Id)
	ttcontroller.RunPlayground(ctx)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for >4 models, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUS145_GetPlaygroundModels_Unauthorized(t *testing.T) {
	ctx, rec := newTTContext(t, http.MethodGet, "/tt/playground/models", nil, 0)
	ttcontroller.GetPlaygroundModels(ctx)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestUS145_GetPlaygroundModels_OK(t *testing.T) {
	user := createBillingTestUser(t, "US145M", 1000)
	ctx, rec := newTTContext(t, http.MethodGet, "/tt/playground/models", nil, user.Id)
	ttcontroller.GetPlaygroundModels(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var wrap struct {
		Data []map[string]any `json:"data"`
	}
	if err := common.Unmarshal(rec.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if wrap.Data == nil {
		t.Fatal("expected data array (may be empty)")
	}
}

func TestUS146_GetSSOProviders_OK(t *testing.T) {
	ctx, rec := newTTContext(t, http.MethodGet, "/tt/sso/providers", nil, 0)
	ttcontroller.GetSSOProviders(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var wrap struct {
		Data []any `json:"data"`
	}
	if err := common.Unmarshal(rec.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if wrap.Data == nil {
		t.Fatal("expected data key")
	}
}

func TestUS146_CreateSSOConfig_InvalidJSONRejected(t *testing.T) {
	user := createBillingTestUser(t, "US146", 1000)
	ctx, rec := newTTContext(t, http.MethodPost, "/tt/sso/admin", []byte("{"), user.Id)
	ttcontroller.CreateSSOConfig(ctx)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid JSON, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUS147_GetSLAStatus_Unauthorized(t *testing.T) {
	ctx, rec := newTTContext(t, http.MethodGet, "/tt/sla/status", nil, 0)
	ttcontroller.GetSLAStatus(ctx)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestUS147_GetSLAStatus_DefaultsShape(t *testing.T) {
	user := createBillingTestUser(t, "US147", 1000)
	ctx, rec := newTTContext(t, http.MethodGet, "/tt/sla/status", nil, user.Id)
	ttcontroller.GetSLAStatus(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		CurrentAvailability float64 `json:"current_availability"`
		TargetAvailability  float64 `json:"target_availability"`
		Status              string  `json:"status"`
	}
	if err := common.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TargetAvailability <= 0 || resp.Status == "" {
		t.Fatalf("unexpected SLA payload: %+v", resp)
	}
}

func TestUS147_GetSLATiers_OK(t *testing.T) {
	user := createBillingTestUser(t, "US147T", 1000)
	ctx, rec := newTTContext(t, http.MethodGet, "/tt/sla/tiers", nil, user.Id)
	ttcontroller.GetSLATiers(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var wrap struct {
		Data []map[string]any `json:"data"`
	}
	if err := common.Unmarshal(rec.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(wrap.Data) < 3 {
		t.Fatalf("expected tier list, got %d", len(wrap.Data))
	}
}
