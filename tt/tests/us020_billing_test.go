package tests

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	ttmodel "github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func createBillingTestUser(t *testing.T, prefix string, quota int) *model.User {
	t.Helper()

	seq := nextAffCode(prefix)
	user := &model.User{
		Username: "user-" + seq,
		Email:    seq + "@example.com",
		AffCode:  seq,
		Quota:    quota,
		Status:   1,
	}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user
}

func TestUS020_ViewBalance(t *testing.T) {
	user := &model.User{
		Username: "balanceuser",
		Email:    "balance@example.com",
		AffCode:  nextAffCode("US020"),
		Quota:    500000,
		Status:   1,
	}
	testDB.Create(user)

	ext := &ttmodel.UserExtension{
		UserId:       uint(user.Id),
		TrialBalance: decimal.NewFromFloat(1.0),
		TrialUsed:    decimal.Zero,
	}
	testDB.Create(ext)

	ext, err := ttmodel.GetUserExtension(int(user.Id))
	if err != nil {
		t.Errorf("Failed to get balance: %v", err)
	}

	if ext.TrialBalance.Cmp(decimal.NewFromFloat(1.0)) != 0 {
		t.Errorf("Expected trial balance $1.0, got %s", ext.TrialBalance.String())
	}

	t.Logf("✓ US-020: View balance test passed")
}

func TestUS021_RechargeBalance(t *testing.T) {
	user := createBillingTestUser(t, "US021", 0)

	err := ttmodel.AdjustUserBalance(uint(user.Id), decimal.NewFromFloat(2.0), "test recharge")
	if err != nil {
		t.Fatalf("recharge should succeed: %v", err)
	}

	var refreshed model.User
	if err := testDB.First(&refreshed, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if refreshed.Quota != 1000000 {
		t.Fatalf("expected quota 1000000 after $2 recharge, got %d", refreshed.Quota)
	}
}

func TestUS022_ConsumeBalance(t *testing.T) {
	user := createBillingTestUser(t, "US022", 1500000)

	err := ttmodel.AdjustUserBalance(uint(user.Id), decimal.NewFromFloat(-1.0), "test consume")
	if err != nil {
		t.Fatalf("consume should succeed: %v", err)
	}

	var refreshed model.User
	if err := testDB.First(&refreshed, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if refreshed.Quota != 1000000 {
		t.Fatalf("expected quota 1000000 after $1 consume, got %d", refreshed.Quota)
	}
}

func TestUS022_ConsumeBalanceInsufficient(t *testing.T) {
	user := createBillingTestUser(t, "US022NEG", 100000)
	initialQuota := user.Quota

	err := ttmodel.AdjustUserBalance(uint(user.Id), decimal.NewFromFloat(-1.0), "test consume insufficient")
	if err == nil {
		t.Fatal("expected consume to fail when balance is insufficient")
	}

	var refreshed model.User
	if err := testDB.First(&refreshed, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if refreshed.Quota != initialQuota {
		t.Fatalf("expected quota unchanged on failed consume, got %d want %d", refreshed.Quota, initialQuota)
	}
}

func TestUS023_RequestRefund(t *testing.T) {
	user := createBillingTestUser(t, "US023", 500000)

	if err := ttmodel.AdjustUserBalance(uint(user.Id), decimal.NewFromFloat(-0.4), "test consume before refund"); err != nil {
		t.Fatalf("pre-refund consume should succeed: %v", err)
	}
	if err := ttmodel.AdjustUserBalance(uint(user.Id), decimal.NewFromFloat(0.3), "test refund"); err != nil {
		t.Fatalf("refund credit should succeed: %v", err)
	}

	var refreshed model.User
	if err := testDB.First(&refreshed, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if refreshed.Quota != 450000 {
		t.Fatalf("expected quota 450000 after consume+refund flow, got %d", refreshed.Quota)
	}
}

func TestUS023_RequestRefundUserNotFound(t *testing.T) {
	err := ttmodel.AdjustUserBalance(999999, decimal.NewFromFloat(0.5), "test refund user not found")
	if err == nil {
		t.Fatal("expected refund adjustment to fail for non-existent user")
	}
}

func TestUS030_ViewUsageStats(t *testing.T) {
	user := createBillingTestUser(t, "US030", 0)
	otherUser := createBillingTestUser(t, "US030OTH", 0)
	now := time.Now()

	records := []ttmodel.ConsumptionRecord{
		{
			UserId:        uint(user.Id),
			RequestId:     "us030-r1",
			Model:         "claude-sonnet-4-6",
			InputTokens:   120,
			OutputTokens:  80,
			ActualCostUSD: decimal.NewFromFloat(0.0012),
			Status:        "completed",
			CreatedAt:     now.Add(-2 * time.Hour),
		},
		{
			UserId:        uint(user.Id),
			RequestId:     "us030-r2",
			Model:         "claude-haiku",
			InputTokens:   40,
			OutputTokens:  10,
			ActualCostUSD: decimal.NewFromFloat(0.0003),
			Status:        "completed",
			CreatedAt:     now.Add(-1 * time.Hour),
		},
		{
			// Should be excluded by startTime filter.
			UserId:        uint(user.Id),
			RequestId:     "us030-old",
			Model:         "claude-sonnet-4-6",
			InputTokens:   999,
			OutputTokens:  999,
			ActualCostUSD: decimal.NewFromFloat(9.999),
			Status:        "completed",
			CreatedAt:     now.Add(-72 * time.Hour),
		},
		{
			// Should be excluded by user isolation.
			UserId:        uint(otherUser.Id),
			RequestId:     "us030-other-user",
			Model:         "claude-sonnet-4-6",
			InputTokens:   500,
			OutputTokens:  500,
			ActualCostUSD: decimal.NewFromFloat(1.5),
			Status:        "completed",
			CreatedAt:     now.Add(-1 * time.Hour),
		},
	}
	for i := range records {
		if err := testDB.Create(&records[i]).Error; err != nil {
			t.Fatalf("failed to create usage record %d: %v", i, err)
		}
	}

	stats, err := ttmodel.GetUserUsage(user.Id, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("usage stats query should succeed: %v", err)
	}

	if stats.InputTokens != 160 {
		t.Fatalf("expected input_tokens=160, got %d", stats.InputTokens)
	}
	if stats.OutputTokens != 90 {
		t.Fatalf("expected output_tokens=90, got %d", stats.OutputTokens)
	}
	if stats.TotalCost != "0.001500" {
		t.Fatalf("expected total_cost=0.001500, got %s", stats.TotalCost)
	}
	if stats.Currency != "USD" {
		t.Fatalf("expected currency=USD, got %s", stats.Currency)
	}
	if len(stats.ByModel) != 2 {
		t.Fatalf("expected 2 models in breakdown, got %d", len(stats.ByModel))
	}

	sonnet, ok := stats.ByModel["claude-sonnet-4-6"]
	if !ok {
		t.Fatal("expected model breakdown for claude-sonnet-4-6")
	}
	if sonnet.InputTokens != 120 || sonnet.OutputTokens != 80 || sonnet.Cost != "0.001200" {
		t.Fatalf("unexpected sonnet breakdown: %+v", sonnet)
	}

	haiku, ok := stats.ByModel["claude-haiku"]
	if !ok {
		t.Fatal("expected model breakdown for claude-haiku")
	}
	if haiku.InputTokens != 40 || haiku.OutputTokens != 10 || haiku.Cost != "0.000300" {
		t.Fatalf("unexpected haiku breakdown: %+v", haiku)
	}
}

func TestUS031_ViewUsageDetails(t *testing.T) {
	user := createBillingTestUser(t, "US031", 0)
	otherUser := createBillingTestUser(t, "US031OTH", 0)
	now := time.Now()

	records := []ttmodel.ConsumptionRecord{
		{
			UserId:        uint(user.Id),
			RequestId:     "us031-1",
			Model:         "claude-sonnet-4-6",
			InputTokens:   10,
			OutputTokens:  5,
			ActualCostUSD: decimal.NewFromFloat(0.0002),
			Status:        "completed",
			CreatedAt:     now.Add(-3 * time.Minute),
		},
		{
			UserId:        uint(user.Id),
			RequestId:     "us031-2",
			Model:         "claude-haiku",
			InputTokens:   20,
			OutputTokens:  8,
			ActualCostUSD: decimal.NewFromFloat(0.0004),
			Status:        "completed",
			CreatedAt:     now.Add(-2 * time.Minute),
		},
		{
			UserId:        uint(user.Id),
			RequestId:     "us031-3",
			Model:         "claude-sonnet-4-6",
			InputTokens:   30,
			OutputTokens:  12,
			ActualCostUSD: decimal.NewFromFloat(0.0006),
			Status:        "completed",
			CreatedAt:     now.Add(-1 * time.Minute),
		},
		{
			// Should be excluded by user isolation.
			UserId:        uint(otherUser.Id),
			RequestId:     "us031-other-user",
			Model:         "claude-sonnet-4-6",
			InputTokens:   999,
			OutputTokens:  999,
			ActualCostUSD: decimal.NewFromFloat(1.999),
			Status:        "completed",
			CreatedAt:     now,
		},
	}
	for i := range records {
		if err := testDB.Create(&records[i]).Error; err != nil {
			t.Fatalf("failed to create usage detail record %d: %v", i, err)
		}
	}

	pageDetails, total, err := ttmodel.GetUserUsageDetails(user.Id, "1", "2", "")
	if err != nil {
		t.Fatalf("usage details query should succeed: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total=3 for user-only records, got %d", total)
	}
	if len(pageDetails) != 2 {
		t.Fatalf("expected page size 2, got %d", len(pageDetails))
	}
	if pageDetails[0].Model != "claude-sonnet-4-6" || pageDetails[1].Model != "claude-haiku" {
		t.Fatalf("expected records ordered by created_at desc, got %+v", pageDetails)
	}

	filtered, filteredTotal, err := ttmodel.GetUserUsageDetails(user.Id, "1", "10", "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("model-filtered details query should succeed: %v", err)
	}
	if filteredTotal != 2 {
		t.Fatalf("expected filtered total=2, got %d", filteredTotal)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered records, got %d", len(filtered))
	}
	for _, d := range filtered {
		if d.Model != "claude-sonnet-4-6" {
			t.Fatalf("expected filtered model claude-sonnet-4-6, got %s", d.Model)
		}
	}

	none, noneTotal, err := ttmodel.GetUserUsageDetails(user.Id, "1", "10", "non-existent-model")
	if err != nil {
		t.Fatalf("non-existent model query should not error: %v", err)
	}
	if noneTotal != 0 || len(none) != 0 {
		t.Fatalf("expected empty result for non-existent model, got total=%d len=%d", noneTotal, len(none))
	}
}

func TestUS090_SetBudgetLimit(t *testing.T) {
	user := createBillingTestUser(t, "US090", 1000000)
	uid := uint(user.Id)

	cfg, err := ttmodel.SetBudgetConfig(uid, 10.0, 100.0, 0.75, false, true)
	if err != nil {
		t.Fatalf("SetBudgetConfig: %v", err)
	}
	if cfg.DailyLimit != 10.0 || cfg.MonthlyLimit != 100.0 || cfg.AlertThreshold != 0.75 {
		t.Fatalf("unexpected limits/threshold: %+v", cfg)
	}
	if cfg.NotifyEmail || !cfg.NotifyWebhook {
		t.Fatalf("unexpected notify flags: %+v", cfg)
	}

	loaded, err := ttmodel.GetBudgetConfig(uid)
	if err != nil {
		t.Fatalf("GetBudgetConfig: %v", err)
	}
	if loaded.DailyLimit != 10.0 || loaded.MonthlyLimit != 100.0 || loaded.AlertThreshold != 0.75 {
		t.Fatalf("persisted config mismatch: %+v", loaded)
	}

	cfg2, err := ttmodel.SetBudgetConfig(uid, 1.0, 2.0, 0.9, true, false)
	if err != nil {
		t.Fatalf("second SetBudgetConfig: %v", err)
	}
	if cfg2.DailyLimit != 1.0 || cfg2.MonthlyLimit != 2.0 || cfg2.AlertThreshold != 0.9 {
		t.Fatalf("update did not apply: %+v", cfg2)
	}
	if !cfg2.NotifyEmail || cfg2.NotifyWebhook {
		t.Fatalf("notify flags not updated: %+v", cfg2)
	}
}

func TestUS091_ViewBudgetStatus(t *testing.T) {
	user := createBillingTestUser(t, "US091", 1000000)
	uid := uint(user.Id)
	if _, err := ttmodel.SetBudgetConfig(uid, 20.0, 200.0, 0.8, true, true); err != nil {
		t.Fatalf("SetBudgetConfig: %v", err)
	}

	rec := &ttmodel.ConsumptionRecord{
		UserId:        uid,
		RequestId:     "us091-" + nextAffCode("req"),
		Model:         "claude-haiku",
		ActualCostUSD: decimal.NewFromFloat(5.0),
		Status:        "completed",
	}
	if err := testDB.Create(rec).Error; err != nil {
		t.Fatalf("create consumption: %v", err)
	}

	status, err := ttmodel.GetBudgetStatus(uid)
	if err != nil {
		t.Fatalf("GetBudgetStatus: %v", err)
	}
	if status.DailyUsed < 5.0-1e-6 || status.MonthlyUsed < 5.0-1e-6 {
		t.Fatalf("expected usage >= 5, got daily=%f monthly=%f", status.DailyUsed, status.MonthlyUsed)
	}
	wantDailyPct := 5.0 / 20.0
	if status.DailyPercent < wantDailyPct-1e-9 || status.DailyPercent > wantDailyPct+1e-9 {
		t.Fatalf("expected daily_percent %f, got %f", wantDailyPct, status.DailyPercent)
	}
	if status.DailyExceeded || status.MonthlyExceeded {
		t.Fatalf("under cap must not set exceeded flags: %+v", status)
	}
	if status.DailyLimit != 20.0 || status.MonthlyLimit != 200.0 || status.AlertThreshold != 0.8 {
		t.Fatalf("unexpected limits in status: %+v", status)
	}
}

func TestUS092_BudgetExceededSignaledInStatus(t *testing.T) {
	userD := createBillingTestUser(t, "US092D", 1000000)
	uidD := uint(userD.Id)
	if _, err := ttmodel.SetBudgetConfig(uidD, 1.0, 0, 0.8, true, true); err != nil {
		t.Fatalf("SetBudgetConfig daily cap: %v", err)
	}
	if err := testDB.Create(&ttmodel.ConsumptionRecord{
		UserId:        uidD,
		RequestId:     "us092d-" + nextAffCode("req"),
		Model:         "claude-haiku",
		ActualCostUSD: decimal.NewFromFloat(1.0),
		Status:        "completed",
	}).Error; err != nil {
		t.Fatalf("create consumption: %v", err)
	}
	stD, err := ttmodel.GetBudgetStatus(uidD)
	if err != nil {
		t.Fatalf("GetBudgetStatus: %v", err)
	}
	if !stD.DailyExceeded {
		t.Fatalf("expected daily_exceeded when usage meets daily cap, got %+v", stD)
	}
	if stD.MonthlyExceeded {
		t.Fatalf("monthly_limit=0 must not set monthly_exceeded, got %+v", stD)
	}

	userM := createBillingTestUser(t, "US092M", 1000000)
	uidM := uint(userM.Id)
	if _, err := ttmodel.SetBudgetConfig(uidM, 0, 3.0, 0.8, true, true); err != nil {
		t.Fatalf("SetBudgetConfig monthly cap: %v", err)
	}
	if err := testDB.Create(&ttmodel.ConsumptionRecord{
		UserId:        uidM,
		RequestId:     "us092m-" + nextAffCode("req"),
		Model:         "claude-haiku",
		ActualCostUSD: decimal.NewFromFloat(3.0),
		Status:        "completed",
	}).Error; err != nil {
		t.Fatalf("create consumption: %v", err)
	}
	stM, err := ttmodel.GetBudgetStatus(uidM)
	if err != nil {
		t.Fatalf("GetBudgetStatus: %v", err)
	}
	if !stM.MonthlyExceeded {
		t.Fatalf("expected monthly_exceeded when usage meets monthly cap, got %+v", stM)
	}
	if stM.DailyExceeded {
		t.Fatalf("daily_limit=0 must not set daily_exceeded, got %+v", stM)
	}
}

func TestUS092_PreConsumeBilling_BlocksPersonalRelayWhenBudgetExceeded(t *testing.T) {
	user := createBillingTestUser(t, "US092PC", 1000000)
	uid := uint(user.Id)
	if _, err := ttmodel.SetBudgetConfig(uid, 1.0, 0, 0.8, true, true); err != nil {
		t.Fatalf("SetBudgetConfig: %v", err)
	}
	if err := testDB.Create(&ttmodel.ConsumptionRecord{
		UserId:        uid,
		RequestId:     "us092pc-" + nextAffCode("req"),
		Model:         "claude-haiku",
		ActualCostUSD: decimal.NewFromFloat(1.0),
		Status:        "completed",
	}).Error; err != nil {
		t.Fatalf("create consumption: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	relayInfo := &relaycommon.RelayInfo{UserId: int(user.Id), TeamId: 0}
	apiErr := service.PreConsumeBilling(c, 0, relayInfo)
	if apiErr == nil {
		t.Fatal("expected PreConsumeBilling to block when personal daily budget exceeded")
	}
	if apiErr.GetErrorCode() != types.ErrorCodeBudgetExceededDaily {
		t.Fatalf("error code: want %q got %q", types.ErrorCodeBudgetExceededDaily, apiErr.GetErrorCode())
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("HTTP status: want 403 got %d", apiErr.StatusCode)
	}
}

func TestUS092_PreConsumeBilling_TeamRelaySkipsPersonalBudgetGate(t *testing.T) {
	owner := createBillingTestUser(t, "US092TM", 1000000)
	uid := uint(owner.Id)
	if _, err := ttmodel.SetBudgetConfig(uid, 1.0, 0, 0.8, true, true); err != nil {
		t.Fatalf("SetBudgetConfig: %v", err)
	}
	if err := testDB.Create(&ttmodel.ConsumptionRecord{
		UserId:        uid,
		RequestId:     "us092tm-" + nextAffCode("req"),
		Model:         "claude-haiku",
		ActualCostUSD: decimal.NewFromFloat(1.0),
		Status:        "completed",
	}).Error; err != nil {
		t.Fatalf("create consumption: %v", err)
	}
	st, err := ttmodel.GetBudgetStatus(uid)
	if err != nil || !st.DailyExceeded {
		t.Fatalf("expected personal daily exceeded before team relay test, err=%v status=%+v", err, st)
	}

	team, err := ttmodel.CreateTeam(uid, "US092 Team", "", 0)
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if err := ttmodel.AdjustTeamBalance(team.Id, 10.0); err != nil {
		t.Fatalf("AdjustTeamBalance: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	relayInfo := &relaycommon.RelayInfo{UserId: int(owner.Id), TeamId: int(team.Id)}
	apiErr := service.PreConsumeBilling(c, 0, relayInfo)
	if apiErr != nil {
		t.Fatalf("team relay must not be blocked by member personal budget: %v", apiErr)
	}
	if relayInfo.Billing == nil {
		t.Fatal("expected BillingSession on relayInfo after successful PreConsumeBilling")
	}
}

func TestUS110_ListCallLogs(t *testing.T) {
	user := createBillingTestUser(t, "US110", 1000000)
	uid := uint(user.Id)

	// Mirrors HTTP handler default: only rows with created_at >= now-7d (see tt/controller/user.go GetCallLogs).
	startWindow := time.Now().AddDate(0, 0, -7)

	oldRec := &ttmodel.ConsumptionRecord{
		UserId:        uid,
		RequestId:     "us110-old-" + nextAffCode("req"),
		Model:         "legacy-model",
		InputTokens:   1,
		OutputTokens:  1,
		ActualCostUSD: decimal.NewFromFloat(0.001),
		Status:        "completed",
	}
	if err := testDB.Create(oldRec).Error; err != nil {
		t.Fatalf("create old record: %v", err)
	}
	oldTime := time.Now().AddDate(0, 0, -10)
	if err := testDB.Model(oldRec).Update("created_at", oldTime).Error; err != nil {
		t.Fatalf("stamp old created_at: %v", err)
	}

	newRec := &ttmodel.ConsumptionRecord{
		UserId:        uid,
		RequestId:     "us110-new-" + nextAffCode("req"),
		Model:         "claude-sonnet-4-6",
		InputTokens:   100,
		OutputTokens:  50,
		ActualCostUSD: decimal.NewFromFloat(0.001),
		Status:        "completed",
	}
	if err := testDB.Create(newRec).Error; err != nil {
		t.Fatalf("create new record: %v", err)
	}
	if err := testDB.Model(newRec).Update("created_at", time.Now().AddDate(0, 0, -1)).Error; err != nil {
		t.Fatalf("stamp new created_at: %v", err)
	}

	logs, total, err := ttmodel.GetCallLogs(uid, "1", "20", "", &startWindow, nil)
	if err != nil {
		t.Fatalf("GetCallLogs: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1 in 7d window, got %d", total)
	}
	if len(logs) != 1 || logs[0].RequestId != newRec.RequestId || logs[0].Model != "claude-sonnet-4-6" {
		t.Fatalf("unexpected list row: %+v", logs)
	}

	other := createBillingTestUser(t, "US110B", 1000000)
	_, otherTotal, err := ttmodel.GetCallLogs(uint(other.Id), "1", "20", "", nil, nil)
	if err != nil {
		t.Fatalf("GetCallLogs other user: %v", err)
	}
	if otherTotal != 0 {
		t.Fatalf("expected 0 logs for other user, got %d", otherTotal)
	}

	t.Logf("✓ US-110: List call logs test passed")
}

func TestUS111_ViewCallLogDetail(t *testing.T) {
	user := createBillingTestUser(t, "US111", 1000000)
	uid := uint(user.Id)
	rec := &ttmodel.ConsumptionRecord{
		UserId:           uid,
		RequestId:        "us111-" + nextAffCode("req"),
		Model:            "claude-opus",
		ChannelId:        42,
		InputTokens:      10,
		OutputTokens:     20,
		CacheReadTokens:  3,
		CacheWriteTokens: 4,
		InputPrice:       decimal.NewFromFloat(1.5),
		OutputPrice:      decimal.NewFromFloat(2.5),
		PreDeductUSD:     decimal.NewFromFloat(0.01),
		ActualCostUSD:    decimal.NewFromFloat(0.02),
		BalanceSource:    "trial",
		Status:           "completed",
	}
	if err := testDB.Create(rec).Error; err != nil {
		t.Fatalf("create consumption: %v", err)
	}

	detail, err := ttmodel.GetCallLogDetail(uid, rec.Id)
	if err != nil {
		t.Fatalf("GetCallLogDetail: %v", err)
	}
	if detail.RequestId != rec.RequestId || detail.Model != rec.Model || detail.ChannelId != rec.ChannelId {
		t.Fatalf("detail mismatch: %+v", detail)
	}
	if detail.InputPrice == "" || detail.OutputPrice == "" || detail.PreDeductUSD == "" || detail.ActualCostUSD == "" {
		t.Fatalf("expected non-empty price/cost strings, got %+v", detail)
	}
	if detail.BalanceSource != "trial" || detail.Status != "completed" {
		t.Fatalf("unexpected detail fields: %+v", detail)
	}

	other := createBillingTestUser(t, "US111X", 1000000)
	_, err = ttmodel.GetCallLogDetail(uint(other.Id), rec.Id)
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound for other user, got %v", err)
	}

	_, err = ttmodel.GetCallLogDetail(uid, 999_999_999)
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound for missing id, got %v", err)
	}

	t.Logf("✓ US-110: call log detail (merged story) test passed")
}
