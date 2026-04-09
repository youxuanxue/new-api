package tests

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	ttmodel "github.com/QuantumNous/new-api/model"
	ttcontroller "github.com/QuantumNous/new-api/tt/controller"
	ttservice "github.com/QuantumNous/new-api/tt/service"
	"github.com/shopspring/decimal"
)

func TestUS100_AutoModelSelection_CodeTask(t *testing.T) {
	r := ttservice.NewSmartRouter(nil)
	out := r.RouteRequest(&ttservice.RouterRequest{
		Model: "auto",
		Messages: []ttservice.RouterMessage{
			{Role: "user", Content: "func a() {}\nfunc b() {}\ndebug stack trace error"},
		},
	})
	if want := "claude-sonnet-4-6"; out != want {
		t.Fatalf("code-heavy auto request: want model %q, got %q", want, out)
	}
}

func TestUS100_AutoModelSelection_SimpleQA(t *testing.T) {
	r := ttservice.NewSmartRouter(nil)
	out := r.RouteRequest(&ttservice.RouterRequest{
		Model: "auto",
		Messages: []ttservice.RouterMessage{
			{Role: "user", Content: "what is golang what is rust"},
		},
	})
	if want := "claude-haiku"; out != want {
		t.Fatalf("simple-QA auto request: want model %q, got %q", want, out)
	}
}

func TestUS100_ExplicitModelPassthrough(t *testing.T) {
	r := ttservice.NewSmartRouter(nil)
	pinned := "gpt-4.1-mini"
	out := r.RouteRequest(&ttservice.RouterRequest{
		Model: pinned,
		Messages: []ttservice.RouterMessage{
			{Role: "user", Content: "hello"},
		},
	})
	if out != pinned {
		t.Fatalf("explicit model: want %q unchanged, got %q", pinned, out)
	}
}

func TestUS101_GetSmartRouterConfig(t *testing.T) {
	ctx, rec := newTTContext(t, http.MethodGet, "/tt/router/config", nil, 1)
	ttcontroller.GetSmartRouterConfig(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Enabled          bool   `json:"enabled"`
		DefaultModel     string `json:"default_model"`
		CodeModel        string `json:"code_model"`
		SimpleQAModel    string `json:"simple_qa_model"`
		LongContextModel string `json:"long_context_model"`
	}
	if err := common.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if !resp.Enabled {
		t.Fatal("expected enabled=true for default smart router")
	}
	if resp.DefaultModel == "" || resp.CodeModel == "" || resp.SimpleQAModel == "" || resp.LongContextModel == "" {
		t.Fatalf("expected non-empty model fields, got %+v", resp)
	}
}

func TestUS101_SmartRouterDisabledUsesDefaultOnly(t *testing.T) {
	r := ttservice.NewSmartRouter(&ttservice.SmartRouterConfig{
		DefaultModel:       "only-default",
		CodeModel:          "code-m",
		SimpleQAModel:      "simple-m",
		LongContextModel:   "long-m",
		EnableSmartRouting: false,
	})
	stats := r.GetRoutingStats()
	if stats["enabled"].(bool) {
		t.Fatal("expected enabled=false")
	}
	out := r.RouteRequest(&ttservice.RouterRequest{
		Model: "auto",
		Messages: []ttservice.RouterMessage{
			{Role: "user", Content: "what is x what is y"},
		},
	})
	if out != "only-default" {
		t.Fatalf("when routing disabled, auto should fall back to default model: got %q", out)
	}
}

func TestUS102_SmartRouteRecommendation(t *testing.T) {
	body, err := common.Marshal(map[string]any{
		"model": "auto",
		"messages": []map[string]string{
			{"role": "user", "content": "what is AI what is ML"},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, rec := newTTContext(t, http.MethodPost, "/tt/router/recommend", body, 1)
	ttcontroller.SmartRoute(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		RecommendedModel string `json:"recommended_model"`
		OriginalModel    string `json:"original_model"`
	}
	if err := common.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.OriginalModel != "auto" {
		t.Fatalf("expected original_model=auto, got %q", resp.OriginalModel)
	}
	if resp.RecommendedModel == "" || resp.RecommendedModel == "auto" {
		t.Fatalf("expected concrete recommended_model, got %q", resp.RecommendedModel)
	}
}

func TestUS102_SmartRouteUnauthorized(t *testing.T) {
	body, err := common.Marshal(map[string]any{"model": "auto", "messages": []any{}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, rec := newTTContext(t, http.MethodPost, "/tt/router/recommend", body, 0)
	ttcontroller.SmartRoute(ctx)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401 for missing user id, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUS102_SmartRouteInvalidJSON(t *testing.T) {
	ctx, rec := newTTContext(t, http.MethodPost, "/tt/router/recommend", []byte("{"), 1)
	ttcontroller.SmartRoute(ctx)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for invalid JSON, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUS120_AdminLogin(t *testing.T) {
	admin := &ttmodel.Admin{
		Username:     "testadmin",
		Email:        "admin@example.com",
		PasswordHash: "hashed_password",
		Role:         ttmodel.RoleSuperAdmin,
		IsActive:     true,
	}
	if err := testDB.Create(admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	result, err := ttmodel.GetAdminByUsername("testadmin")
	if err != nil {
		t.Fatalf("GetAdminByUsername: %v", err)
	}
	if result.Username != "testadmin" || result.Role != ttmodel.RoleSuperAdmin || !result.IsActive {
		t.Fatalf("unexpected admin row: %+v", result)
	}
}

func TestUS120_GetAdminByUsername_NotFound(t *testing.T) {
	_, err := ttmodel.GetAdminByUsername("us120-missing-" + nextAffCode("NA"))
	if err == nil {
		t.Fatal("expected error for missing admin username")
	}
}

func TestUS121_ListUsersForAdmin_ReturnsSeededUsers(t *testing.T) {
	u1 := &ttmodel.User{Username: "us121a-" + nextAffCode("U"), Email: nextAffCode("E") + "-121a@example.com", AffCode: nextAffCode("121A"), Status: 1, Quota: 500_000}
	u2 := &ttmodel.User{Username: "us121b-" + nextAffCode("U"), Email: nextAffCode("E") + "-121b@example.com", AffCode: nextAffCode("121B"), Status: 1, Quota: 1_000_000}
	if err := testDB.Create(u1).Error; err != nil {
		t.Fatalf("seed u1: %v", err)
	}
	if err := testDB.Create(u2).Error; err != nil {
		t.Fatalf("seed u2: %v", err)
	}

	rows, total, err := ttmodel.ListUsersForAdmin("1", "20", "", "")
	if err != nil {
		t.Fatalf("ListUsersForAdmin: %v", err)
	}
	if total < 2 {
		t.Fatalf("expected total >= 2, got %d", total)
	}
	seen := 0
	for _, r := range rows {
		if r.Id == uint(u1.Id) || r.Id == uint(u2.Id) {
			seen++
		}
	}
	if seen != 2 {
		t.Fatalf("expected both seeded users in page, seen=%d rows=%d", seen, len(rows))
	}
}

func TestUS121_AdjustUserBalance_IncreasesQuota(t *testing.T) {
	u := &ttmodel.User{Username: "us121bal-" + nextAffCode("U"), Email: nextAffCode("E") + "@121bal.example.com", AffCode: nextAffCode("121Q"), Status: 1, Quota: 500_000}
	if err := testDB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := ttmodel.AdjustUserBalance(uint(u.Id), decimal.RequireFromString("1.00"), "test top-up"); err != nil {
		t.Fatalf("AdjustUserBalance: %v", err)
	}
	var got ttmodel.User
	if err := testDB.First(&got, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if got.Quota != 500_000+500_000 {
		t.Fatalf("expected quota 1000000, got %d", got.Quota)
	}
}

func TestUS121_AdjustUserBalance_InsufficientRejected(t *testing.T) {
	u := &ttmodel.User{Username: "us121neg-" + nextAffCode("U"), Email: nextAffCode("E") + "@121neg.example.com", AffCode: nextAffCode("121N"), Status: 1, Quota: 0}
	if err := testDB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	err := ttmodel.AdjustUserBalance(uint(u.Id), decimal.RequireFromString("-0.01"), "should fail")
	if err == nil {
		t.Fatal("expected AdjustUserBalance to fail when quota would go negative")
	}
	if !strings.Contains(err.Error(), "余额") {
		t.Fatalf("expected 余额不足 style error, got %v", err)
	}
}

func TestUS121_RecordAdminAudit_AppearsInList(t *testing.T) {
	ctx, _ := newTTContext(t, http.MethodPost, "/admin/users/1/adjust-balance", nil, 1)
	ttmodel.RecordAdminAudit(1, "adjust_balance", "42", "user", ctx)

	logs, total, err := ttmodel.ListAuditLogsForAdmin("1", "20", "", "", "", "")
	if err != nil {
		t.Fatalf("ListAuditLogsForAdmin: %v", err)
	}
	if total < 1 || len(logs) < 1 {
		t.Fatalf("expected at least one audit row, total=%d len=%d", total, len(logs))
	}
	found := false
	for _, l := range logs {
		if l.Operation == "adjust_balance" && l.TargetId == "42" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected adjust_balance / target 42 in logs, got %+v", logs)
	}
}

func TestUS122_ChannelCreateUpdateDelete(t *testing.T) {
	raw, err := ttmodel.CreateChannelByAdmin("us122-ch", "openai", "sk-test-key", "", "gpt-4", 10)
	if err != nil {
		t.Fatalf("CreateChannelByAdmin: %v", err)
	}
	ch, ok := raw.(ttmodel.Channel)
	if !ok || ch.Id == 0 {
		t.Fatalf("expected Channel with id, got %#v", raw)
	}

	list, err := ttmodel.ListChannelsForAdmin()
	if err != nil {
		t.Fatalf("ListChannelsForAdmin: %v", err)
	}
	found := false
	for _, it := range list {
		c, ok := it.(ttmodel.Channel)
		if ok && c.Id == ch.Id {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("created channel not present in list")
	}

	if err := ttmodel.UpdateChannelByAdmin(uint(ch.Id), "us122-renamed", "", "", "gpt-4o", 0, ""); err != nil {
		t.Fatalf("UpdateChannelByAdmin: %v", err)
	}
	var updated ttmodel.Channel
	if err := testDB.First(&updated, ch.Id).Error; err != nil {
		t.Fatalf("reload channel: %v", err)
	}
	if updated.Name != "us122-renamed" {
		t.Fatalf("expected renamed channel, got %q", updated.Name)
	}

	if _, err := ttmodel.TestChannelByAdmin(uint(ch.Id)); err != nil {
		t.Fatalf("TestChannelByAdmin: %v", err)
	}

	if err := ttmodel.DeleteChannelByAdmin(uint(ch.Id)); err != nil {
		t.Fatalf("DeleteChannelByAdmin: %v", err)
	}
	if _, err := ttmodel.TestChannelByAdmin(uint(ch.Id)); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestUS122_TestChannelByAdmin_NotFound(t *testing.T) {
	_, err := ttmodel.TestChannelByAdmin(9_999_001)
	if err == nil || !strings.Contains(err.Error(), "渠道") {
		t.Fatalf("expected 渠道不存在, got %v", err)
	}
}

func TestUS123_PoolAddDuplicateRejected(t *testing.T) {
	email := "pool-" + nextAffCode("E") + "@example.com"
	if _, err := ttmodel.AddPoolAccount(email, "pw", ""); err != nil {
		t.Fatalf("first AddPoolAccount: %v", err)
	}
	_, errDup := ttmodel.AddPoolAccount(email, "pw2", "")
	if errDup == nil {
		t.Fatal("expected duplicate email to be rejected")
	}
	if !strings.Contains(errDup.Error(), "邮箱") {
		t.Fatalf("expected duplicate email message, got %v", errDup)
	}

	st, err := ttmodel.GetPoolStatus()
	if err != nil {
		t.Fatalf("GetPoolStatus: %v", err)
	}
	if st.TotalAccounts < 1 {
		t.Fatalf("expected pool total >= 1, got %+v", st)
	}
}

func TestUS124_CreateUpdateModelPricing(t *testing.T) {
	name := "us124-model-" + nextAffCode("M")
	p, err := ttmodel.CreateModelPricing(name, "1.5", "2.5", "", "", "")
	if err != nil {
		t.Fatalf("CreateModelPricing: %v", err)
	}
	if p.Model != name || !p.InputPrice.Equal(decimal.RequireFromString("1.5")) {
		t.Fatalf("unexpected pricing row: %+v", p)
	}

	if err := ttmodel.UpdateModelPricing(p.Id, "2", "", "", "", ""); err != nil {
		t.Fatalf("UpdateModelPricing: %v", err)
	}
	var row ttmodel.ModelPricing
	if err := testDB.First(&row, p.Id).Error; err != nil {
		t.Fatalf("reload pricing: %v", err)
	}
	if !row.InputPrice.Equal(decimal.RequireFromString("2")) {
		t.Fatalf("expected input_price=2, got %s", row.InputPrice.String())
	}
}

func TestUS124_CreateModelPricing_DuplicateModelRejected(t *testing.T) {
	name := "us124-dup-" + nextAffCode("M")
	if _, err := ttmodel.CreateModelPricing(name, "1", "1", "", "", ""); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := ttmodel.CreateModelPricing(name, "2", "2", "", "", ""); err == nil {
		t.Fatal("expected duplicate model name to fail")
	}
}

func TestUS125_ViewAdminDashboard(t *testing.T) {
	dashboard, err := ttmodel.GetDashboardData()
	if err != nil {
		t.Fatalf("GetDashboardData: %v", err)
	}
	if dashboard.APIAvailability == "" || dashboard.PoolAvailability == "" {
		t.Fatalf("expected availability fields set, got api=%q pool=%q", dashboard.APIAvailability, dashboard.PoolAvailability)
	}
	if len(dashboard.TrendData.Dates) != 7 || len(dashboard.TrendData.Requests) != 7 {
		t.Fatalf("expected 7-day trend, got dates=%d requests=%d", len(dashboard.TrendData.Dates), len(dashboard.TrendData.Requests))
	}
}

func TestUS126_ListAuditLogsForAdmin_ReturnsRows(t *testing.T) {
	log := ttmodel.AdminAuditLog{
		AdminId:     7,
		AdminName:   "admin_7",
		Operation:   "us126_test_op",
		TargetId:    "99",
		TargetType:  "user",
		IP:          "127.0.0.1",
		TOTPVerified: false,
	}
	if err := testDB.Create(&log).Error; err != nil {
		t.Fatalf("seed audit log: %v", err)
	}
	logs, total, err := ttmodel.ListAuditLogsForAdmin("1", "20", "", "", "", "")
	if err != nil {
		t.Fatalf("ListAuditLogsForAdmin: %v", err)
	}
	if total < 1 {
		t.Fatalf("expected total >= 1, got %d", total)
	}
	found := false
	for _, l := range logs {
		if l.Operation == "us126_test_op" && l.TargetId == "99" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected seeded operation in list, got %+v", logs)
	}
}
